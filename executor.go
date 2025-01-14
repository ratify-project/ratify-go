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
	"fmt"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/ratify-project/ratify-go/internal/stack"
	"oras.land/oras-go/v2/registry"
)

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
	// Executor could use multiple verifiers to validate artifacts. Required.
	Verifiers []Verifier

	// Executor should configure exactly one store to fetch supply chain
	// content. Required.
	Store Store

	// Executor should have at most one policy enforcer to evalute reports. If
	// not set, the validation result will be returned without evaluation.
	// Optional.
	PolicyEnforcer PolicyEnforcer
}

// NewExecutor creates a new executor with the given verifiers, store, and
// policy enforcer.
func NewExecutor(verifiers []Verifier, store Store, policyEnforcer PolicyEnforcer) (*Executor, error) {
	if err := validateExecutorSetup(verifiers, store); err != nil {
		return nil, err
	}

	return &Executor{
		Verifiers:      verifiers,
		Store:          store,
		PolicyEnforcer: policyEnforcer,
	}, nil
}

// ValidateArtifact returns the result of verifying an artifact.
func (e *Executor) ValidateArtifact(ctx context.Context, opts ValidateArtifactOptions) (*ValidationResult, error) {
	if err := validateExecutorSetup(e.Verifiers, e.Store); err != nil {
		return nil, err
	}

	aggregatedVerifierReports, err := e.aggregateVerifierReports(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate and aggregate verifier reports: %w", err)
	}

	if e.PolicyEnforcer == nil {
		return &ValidationResult{
			Succeeded:       false,
			ArtifactReports: aggregatedVerifierReports,
		}, nil
	}

	decision, err := e.PolicyEnforcer.Evaluate(ctx, aggregatedVerifierReports)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate verifier reports: %w", err)
	}

	return &ValidationResult{
		Succeeded:       decision,
		ArtifactReports: aggregatedVerifierReports,
	}, nil
}

// aggregateVerifierReports generates and aggregates all verifier reports.
func (e *Executor) aggregateVerifierReports(ctx context.Context, opts ValidateArtifactOptions) ([]*ValidationReport, error) {
	var aggregatedVerifierReports []*ValidationReport

	// Only resolve the root subject reference.
	ref, err := e.resolveSubject(ctx, opts.Subject)
	if err != nil {
		return nil, err
	}

	// TODO: Implement a worker pool to validate artifacts concurrently.
	// TODO: Enforce check on the stack size.
	// Enqueue the subject artifact as the first task.
	taskStack := stack.NewStack[*task]()
	taskStack.Push(&task{
		store:    e.Store,
		artifact: ref.String(),
		registry: ref.Registry,
		repo:     ref.Repository,
	})

	for !taskStack.IsEmpty() {
		// Ignore the bool as the stack is not empty.
		task, _ := taskStack.Pop()

		newTasks, artifactReports, err := e.verifySubjectAgainstReferrers(ctx, task, opts.ReferenceTypes)
		if err != nil {
			return nil, fmt.Errorf("failed to validate artifact %s: %w", task.artifact, err)
		}

		// Push the new tasks to the stack.
		taskStack.Push(newTasks...)

		// If the current task is the root task, add the artifactReports to the
		// aggregatedVerifierReports. Otherwise, they are just artifactReports
		// of the subject artifact.
		if task.subjectReport != nil {
			task.subjectReport.ArtifactReports = append(task.subjectReport.ArtifactReports, artifactReports...)
		} else {
			aggregatedVerifierReports = append(aggregatedVerifierReports, artifactReports...)
		}
	}

	return aggregatedVerifierReports, nil
}

// verifySubjectAgainstReferrers verifies the subject artifact against all 
// referrers in the store and produces new tasks for each referrer.
func (e *Executor) verifySubjectAgainstReferrers(ctx context.Context, currentTask *task, referenceTypes []string) ([]*task, []*ValidationReport, error) {
	var newTasks []*task
	referrers, err := currentTask.store.ListReferrers(ctx, currentTask.artifact, referenceTypes, nil)
	if err != nil {
		return newTasks, nil, fmt.Errorf("failed to list referrers for artifact %s: %w", currentTask.artifact, err)
	}

	// We need to verify the artifact against its required referrer artifacts.
	// artifactReports is used to store the validation reports of those
	// referrer artifacts.
	var artifactReports []*ValidationReport
	for _, referrer := range referrers {
		results, err := e.verifyArtifact(ctx, currentTask.store, currentTask.artifact, referrer)
		if err != nil {
			return newTasks, nil, err
		}
		artifactReport := &ValidationReport{
			Subject:  currentTask.artifact,
			Results:  results,
			Artifact: referrer,
		}
		artifactReports = append(artifactReports, artifactReport)
	
		referrerArtifact := registry.Reference{
			Registry:   currentTask.registry,
			Repository: currentTask.repo,
			Reference:  referrer.Digest.String(),
		}
		newTasks = append(newTasks, &task{
			store:         currentTask.store,
			artifact:      referrerArtifact.String(),
			registry:      currentTask.registry,
			repo:          currentTask.repo,
			subjectReport: artifactReport,
		})
	}
	return newTasks, artifactReports, nil
}

// verifyArtifact verifies the artifact by all configured verifiers and returns
// error if any of the verifier fails.
func (e *Executor) verifyArtifact(ctx context.Context, store Store, subject string, artifact ocispec.Descriptor) ([]*VerificationResult, error) {
	var verifierReports []*VerificationResult

	for _, verifier := range e.Verifiers {
		if !verifier.Verifiable(artifact) {
			continue
		}
		verifierReport, err := verifier.Verify(ctx, store, subject, artifact)
		if err != nil {
			return nil, fmt.Errorf("failed to verify artifact %s with verifier %s: %w", subject, verifier.Name(), err)
		}

		verifierReports = append(verifierReports, verifierReport)
	}

	return verifierReports, nil
}

func (e *Executor) resolveSubject(ctx context.Context, subject string) (registry.Reference, error) {
	ref, err := registry.ParseReference(subject)
	if err != nil {
		return registry.Reference{}, fmt.Errorf("failed to parse subject reference %s: %w", subject, err)
	}

	artifactDesc, err := e.Store.Resolve(ctx, ref.String())
	if err != nil {
		return registry.Reference{}, fmt.Errorf("failed to resolve subject reference %s: %w", ref.Reference, err)
	}
	ref.Reference = artifactDesc.Digest.String()
	return ref, nil
}

func validateExecutorSetup(verifiers []Verifier, store Store) error {
	if len(verifiers) == 0 {
		return fmt.Errorf("at least one verifier must be configured")
	}
	if store == nil {
		return fmt.Errorf("store must be configured")
	}
	return nil
}
