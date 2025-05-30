/*
Copyright The Ratify Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ratify

import (
	"context"
	"errors"
	"fmt"

	"github.com/notaryproject/ratify-go/internal/stack"
	"github.com/notaryproject/ratify-go/internal/worker"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry"
)

// errSubjectPruned is returned when the evaluator does not need given subject
// to be verified to make a decision by [Evaluator.Pruned].
var errSubjectPruned = errors.New("evaluator sub-graph is pruned for the subject")

// ValidateArtifactOptions describes the artifact validation options.
type ValidateArtifactOptions struct {
	// Subject is the reference of the artifact to be validated. Required.
	Subject string

	// ReferenceTypes is a list of reference types that should be verified
	// against in associated artifacts. Empty list means all artifacts should be
	// verified. Optional.
	ReferenceTypes []string
}

// ValidationResult aggregates verifier reports and the final verification
// result evaluated by the policy enforcer.
type ValidationResult struct {
	// Succeeded represents the outcome determined by the policy enforcer based
	// on the aggregated verifier reports. And if an error occurs during the
	// validation process prior to policy evaluation, it will be set to `false`.
	// If the policy enforcer is not set in the executor, this field will be set
	// to `false`. In such cases, this field should be ignored. Required.
	Succeeded bool

	// ArtifactReports is aggregated reports of verifying associated artifacts.
	// This field can be nil if an error occured during validation or no reports
	// were generated. Optional.
	ArtifactReports []*ValidationReport
}

// Executor is defined to validate artifacts.
type Executor struct {
	// Executor should configure exactly one store to fetch supply chain
	// content. Required.
	Store Store

	// Executor could use multiple verifiers to validate artifacts. Required.
	Verifiers []Verifier

	// Executor should have at most one policy enforcer to evalute reports. If
	// not set, the validation result will be returned without evaluation.
	// Optional.
	PolicyEnforcer PolicyEnforcer

	// ConcurrencyLimit is the maximum number of concurrent tasks that can be
	// executed.
	//
	// note: each call to ValidateArtifact will create new worker pools based on this
	// limit.
	ConcurrencyLimit int
}

// NewExecutor creates a new executor with the given verifiers, store, and
// policy enforcer.
func NewExecutor(store Store, verifiers []Verifier, policyEnforcer PolicyEnforcer, concurrencyLimit int) (*Executor, error) {
	if err := validateExecutorSetup(store, verifiers); err != nil {
		return nil, err
	}

	if concurrencyLimit <= 0 {
		return nil, fmt.Errorf("concurrency limit (%d) must be greater than 0", concurrencyLimit)
	}

	return &Executor{
		Store:            store,
		Verifiers:        verifiers,
		PolicyEnforcer:   policyEnforcer,
		ConcurrencyLimit: concurrencyLimit,
	}, nil
}

// ValidateArtifact returns the result of verifying an artifact.
func (e *Executor) ValidateArtifact(ctx context.Context, opts ValidateArtifactOptions) (*ValidationResult, error) {
	if err := validateExecutorSetup(e.Store, e.Verifiers); err != nil {
		return nil, err
	}

	aggregatedVerifierReports, evaluator, err := e.aggregateVerifierReports(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate and aggregate verifier reports: %w", err)
	}

	if evaluator == nil {
		return &ValidationResult{
			Succeeded:       false,
			ArtifactReports: aggregatedVerifierReports,
		}, nil
	}

	decision, err := evaluator.Evaluate(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate verifier reports: %w", err)
	}

	return &ValidationResult{
		Succeeded:       decision,
		ArtifactReports: aggregatedVerifierReports,
	}, nil
}

// aggregateVerifierReports generates and aggregates all verifier reports.
func (e *Executor) aggregateVerifierReports(ctx context.Context, opts ValidateArtifactOptions) ([]*ValidationReport, Evaluator, error) {
	// only resolve the root subject reference.
	ref, desc, err := e.resolveSubject(ctx, opts.Subject)
	if err != nil {
		return nil, nil, err
	}
	repo := ref.Registry + "/" + ref.Repository

	var evaluator Evaluator
	if e.PolicyEnforcer != nil {
		evaluator, err = e.PolicyEnforcer.Evaluator(ctx, ref.Reference)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create a new evaluator: %w", err)
		}
	}

	// create worker pools for referrer and verifier tasks
	referrerTaskPool, err := worker.NewPool(e.ConcurrencyLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create a new referrer worker pool: %w", err)
	}
	defer close(referrerTaskPool)
	verifierTaskPool, err := worker.NewPool(e.ConcurrencyLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create a new verifier worker pool: %w", err)
	}
	defer close(verifierTaskPool)

	rootTask := &executorTask{
		artifact:     ref,
		artifactDesc: desc,
		subjectReport: &ValidationReport{
			Artifact: desc,
		},
	}
	taskStack := stack.Stack[*executorTask]{}
	taskStack.Push(rootTask)
	for !taskStack.IsEmpty() {
		artifactTaskGroup, ctx := worker.NewGroup[[]*executorTask](ctx, e.ConcurrencyLimit)
		// prepare batch processing
		taskBatch := make([]*executorTask, 0, e.ConcurrencyLimit)
		for i := 0; i < e.ConcurrencyLimit && !taskStack.IsEmpty(); i++ {
			task, ok := taskStack.TryPop()
			if !ok {
				break
			}
			taskBatch = append(taskBatch, task)
		}

		// batch process the tasks
		for _, task := range taskBatch {
			artifactTaskGroup.Go(func() ([]*executorTask, error) {
				return e.verifySubjectAgainstReferrers(ctx, task, repo, opts.ReferenceTypes, evaluator, referrerTaskPool, verifierTaskPool)
			})
		}

		// add new tasks to the stack
		newTaskSlice, err := artifactTaskGroup.Wait()
		if err != nil {
			return nil, nil, err

		}
		for _, newTasks := range newTaskSlice {
			for _, newTask := range newTasks {
				if newTask == nil {
					continue
				}
				taskStack.Push(newTask)
			}
		}
	}

	return rootTask.subjectReport.ArtifactReports, evaluator, nil
}

// verifySubjectAgainstReferrers verifies the subject artifact against all
// referrers in the store and produces new tasks for each referrer.
func (e *Executor) verifySubjectAgainstReferrers(parentCtx context.Context, task *executorTask, repo string, referenceTypes []string, evaluator Evaluator, referrerTaskPool, verifierTaskPool worker.Pool) ([]*executorTask, error) {
	artifact := task.artifact.String()

	// We need to verify the artifact against its required referrer artifacts.
	// artifactReports is used to store the validation reports of those
	// referrer artifacts.
	referrerTaskGroup, ctx := worker.NewGroupWithSharedPool[*ValidationReport](parentCtx, referrerTaskPool)
	err := e.Store.ListReferrers(ctx, artifact, referenceTypes, func(referrers []ocispec.Descriptor) error {
		for _, referrer := range referrers {
			referrerTaskGroup.Go(func() (*ValidationReport, error) {
				results, err := e.verifyArtifact(ctx, repo, task.artifactDesc, referrer, evaluator, verifierTaskPool)
				if err != nil {
					if errors.Is(err, errSubjectPruned) && len(results) > 0 {
						// it is possible that one or some verifiers' reports in the
						// results and the next verifier triggers the subject pruned state,
						// so the results are not empty.
						artifactReport := &ValidationReport{
							Subject:  artifact,
							Results:  results,
							Artifact: referrer,
						}
						return artifactReport, errSubjectPruned
					}
					return nil, err
				}

				artifactReport := &ValidationReport{
					Subject:  artifact,
					Results:  results,
					Artifact: referrer,
				}
				return artifactReport, nil
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list referrers artifact %s: %w", artifact, err)
	}

	isSubjectPruned := false
	artifactReports, err := referrerTaskGroup.Wait()
	if err != nil {
		if err != errSubjectPruned {
			return nil, fmt.Errorf("failed to verify referrers for artifact %s: %w", artifact, err)
		}
		isSubjectPruned = true
	}

	if evaluator != nil {
		if err := evaluator.Commit(ctx, task.artifactDesc.Digest.String()); err != nil {
			return nil, fmt.Errorf("failed to commit the artifact %s: %w", artifact, err)
		}
	}
	var newTasks []*executorTask
	if !isSubjectPruned {
		// start processing next level of referrers
		for _, artifactReport := range artifactReports {
			if artifactReport == nil {
				continue
			}
			referrerArtifact := task.artifact
			referrerArtifact.Reference = artifactReport.Artifact.Digest.String()
			newTasks = append(newTasks, &executorTask{
				artifact:      referrerArtifact,
				artifactDesc:  artifactReport.Artifact,
				subjectReport: artifactReport,
			})
			task.subjectReport.ArtifactReports = append(task.subjectReport.ArtifactReports, artifactReport)
		}
	}
	return newTasks, nil
}

// verifyArtifact verifies the artifact by all configured verifiers and returns
// error if any of the verifier fails.
func (e *Executor) verifyArtifact(ctx context.Context, repo string, subjectDesc, artifact ocispec.Descriptor, evaluator Evaluator, verifierTaskPool worker.Pool) ([]*VerificationResult, error) {
	verifierTaskGroup, ctx := worker.NewGroupWithSharedPool[*VerificationResult](ctx, verifierTaskPool)
	for _, verifier := range e.Verifiers {
		if !verifier.Verifiable(artifact) {
			continue
		}

		verifierTaskGroup.Go(func() (*VerificationResult, error) {
			if evaluator != nil {
				prunedState, err := evaluator.Pruned(ctx, subjectDesc.Digest.String(), artifact.Digest.String(), verifier.Name())
				if err != nil {
					return nil, fmt.Errorf("failed to check if verifier: %s is required to verify subject: %s, against artifact: %s, err: %w", verifier.Name(), subjectDesc.Digest, artifact.Digest, err)
				}
				switch prunedState {
				case PrunedStateVerifierPruned:
					// Skip this verifier if it's not required.
					return nil, nil
				case PrunedStateSubjectPruned:
					// Skip remaining verifiers and return `errSubjectPruned` to
					// notify `ListReferrers`stop processing.
					return nil, errSubjectPruned
				default:
					// do nothing if it's not pruned.
				}
			}

			// Verify the subject artifact against the referrer artifact.
			verifierReport, err := verifier.Verify(ctx, &VerifyOptions{
				Store:              e.Store,
				Repository:         repo,
				SubjectDescriptor:  subjectDesc,
				ArtifactDescriptor: artifact,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to verify artifact %s@%s with verifier %s: %w", repo, subjectDesc.Digest, verifier.Name(), err)
			}

			if evaluator != nil {
				if err := evaluator.AddResult(ctx, subjectDesc.Digest.String(), artifact.Digest.String(), verifierReport); err != nil {
					return nil, fmt.Errorf("failed to add verifier report for artifact %s@%s verified by verifier %s: %w", repo, subjectDesc.Digest, verifier.Name(), err)
				}
			}

			return verifierReport, nil
		})
	}
	verificationResults, err := verifierTaskGroup.Wait()
	if err != nil {
		return nil, err
	}
	// Filter out nil results and return the verification results.
	var results []*VerificationResult
	for _, result := range verificationResults {
		if result != nil {
			results = append(results, result)
		}
	}
	return results, nil
}

func (e *Executor) resolveSubject(ctx context.Context, subject string) (registry.Reference, ocispec.Descriptor, error) {
	ref, err := registry.ParseReference(subject)
	if err != nil {
		return registry.Reference{}, ocispec.Descriptor{}, fmt.Errorf("failed to parse subject reference %s: %w", subject, err)
	}

	artifactDesc, err := e.Store.Resolve(ctx, ref.String())
	if err != nil {
		return registry.Reference{}, ocispec.Descriptor{}, fmt.Errorf("failed to resolve subject reference %s: %w", ref.Reference, err)
	}
	ref.Reference = artifactDesc.Digest.String()
	return ref, artifactDesc, nil
}

// executorTask is a struct that represents a executorTask that verifies an artifact by
// the executor.
type executorTask struct {
	// artifact is the digested reference of the referrer artifact that will be
	// verified against.
	artifact registry.Reference

	// artifactDesc is the descriptor of the referrer artifact that will be
	// verified against.
	artifactDesc ocispec.Descriptor

	// subjectReport is the report of the subject artifact.
	subjectReport *ValidationReport
}

func validateExecutorSetup(store Store, verifiers []Verifier) error {
	if store == nil {
		return fmt.Errorf("store must be configured")
	}
	if len(verifiers) == 0 {
		return fmt.Errorf("at least one verifier must be configured")
	}
	return nil
}
