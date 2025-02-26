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
	// Executor should configure exactly one store to fetch supply chain
	// content. Required.
	Store Store

	// Executor could use multiple verifiers to validate artifacts. Required.
	Verifiers []Verifier

	// Executor should have at most one policy enforcer to evalute reports. If
	// not set, the validation result will be returned without evaluation.
	// Optional.
	PolicyEnforcer PolicyEnforcer
}

// NewExecutor creates a new executor with the given verifiers, store, and
// policy enforcer.
func NewExecutor(store Store, verifiers []Verifier, policyEnforcer PolicyEnforcer) (*Executor, error) {
	if err := validateExecutorSetup(store, verifiers); err != nil {
		return nil, err
	}

	return &Executor{
		Store:          store,
		Verifiers:      verifiers,
		PolicyEnforcer: policyEnforcer,
	}, nil
}

// ValidateArtifact returns the result of verifying an artifact.
func (e *Executor) ValidateArtifact(ctx context.Context, opts ValidateArtifactOptions) (*ValidationResult, error) {
	if err := validateExecutorSetup(e.Store, e.Verifiers); err != nil {
		return nil, err
	}

	aggregatedVerifierReports, err := e.aggregateVerifierReports(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate and aggregate verifier reports: %w", err)
	}

	if e.PolicyEnforcer == nil {
		return &ValidationResult{
			Succeeded:       false,
			ArtifactReports: aggregatedVerifierReports.ArtifactReports,
		}, nil
	}

	decision, err := e.PolicyEnforcer.EvaluateReport(ctx, aggregatedVerifierReports)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate verifier reports: %w", err)
	}

	return &ValidationResult{
		Succeeded:       decision == Allow,
		ArtifactReports: aggregatedVerifierReports.ArtifactReports,
	}, nil
}

// aggregateVerifierReports generates and aggregates all verifier reports.
func (e *Executor) aggregateVerifierReports(ctx context.Context, opts ValidateArtifactOptions) (*ValidationReport, error) {
	// Only resolve the root subject reference.
	ref, desc, err := e.resolveSubject(ctx, opts.Subject)
	if err != nil {
		return nil, err
	}
	repo := ref.Registry + "/" + ref.Repository

	// TODO: Implement a worker pool to validate artifacts concurrently.
	// TODO: Enforce check on the stack size.
	// Enqueue the subject artifact as the first task.
	var state EvaluationState
	if e.PolicyEnforcer != nil {
		if state, err = e.PolicyEnforcer.NewEvaluationState(ctx); err != nil {
			return nil, err
		}
	}
	rootTask := &executorTask{
		artifact:     ref,
		artifactDesc: desc,
		subjectReport: &ValidationReport{
			Artifact:              desc,
			policyEvaluationState: state,
		},
	}
	var taskStack stack.Stack[*executorTask]
	taskStack.Push(rootTask)
	for taskStack.Len() > 0 {
		task := taskStack.Pop()

		newTasks, err := e.verifySubjectAgainstReferrers(ctx, task, repo, opts.ReferenceTypes)
		if err != nil {
			return nil, fmt.Errorf("failed to validate artifact %v: %w", task.artifact, err)
		}

		// Push the new tasks to the stack.
		taskStack.Push(newTasks...)
	}

	return rootTask.subjectReport, nil
}

// verifySubjectAgainstReferrers verifies the subject artifact against all
// referrers in the store and produces new tasks for each referrer.
func (e *Executor) verifySubjectAgainstReferrers(ctx context.Context, task *executorTask, repo string, referenceTypes []string) ([]*executorTask, error) {
	artifact := task.artifact.String()

	// We need to verify the artifact against its required referrer artifacts.
	// artifactReports is used to store the validation reports of those
	// referrer artifacts.
	var newTasks []*executorTask
	var artifactReports []*ValidationReport
	err := e.Store.ListReferrers(ctx, artifact, referenceTypes, func(referrers []ocispec.Descriptor) error {
		for _, referrer := range referrers {
			var nextEvalState EvaluationState
			if task.subjectReport.policyEvaluationState != nil {
				nextEvalState = task.subjectReport.policyEvaluationState.NextState(referrer)
			}
			artifactReport := &ValidationReport{
				Subject:               artifact,
				Artifact:              referrer,
				policyEvaluationState: nextEvalState,
			}

			artifactReport, err := e.verifyArtifact(ctx, repo, task.artifactDesc, referrer, artifactReport)
			if err != nil {
				return err
			}

			artifactReports = append(artifactReports, artifactReport)

			referrerArtifact := task.artifact
			referrerArtifact.Reference = referrer.Digest.String()
			newTasks = append(newTasks, &executorTask{
				artifact:      referrerArtifact,
				artifactDesc:  referrer,
				subjectReport: artifactReport,
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to verify referrers for artifact %s: %w", artifact, err)
	}
	task.subjectReport.ArtifactReports = append(task.subjectReport.ArtifactReports, artifactReports...)

	return newTasks, nil
}

// verifyArtifact verifies the artifact by all configured verifiers and returns
// error if any of the verifier fails.
func (e *Executor) verifyArtifact(ctx context.Context, repo string, subjectDesc, artifact ocispec.Descriptor, artifactReport *ValidationReport) (*ValidationReport, error) {
	artifactReport.Results = make([]*VerificationResult, 0)
	for _, verifier := range e.Verifiers {
		if !verifier.Verifiable(artifact) {
			continue
		}
		if e.PolicyEnforcer != nil && e.PolicyEnforcer.AllowEvalDuringVerify() {
			// Also consider combining RequireFurtherVerification and EvaluateResult
			// into one method and pass `verifier` or `verify()` func as a parameter.

			// check if it's verificationRequired to verify the subject artifact.
			verificationRequired, err := e.PolicyEnforcer.RequireFurtherVerification(ctx, artifactReport, verifier)
			if err != nil {
				return nil, fmt.Errorf("failed to check if verifier is needed: %w", err)
			}
			if !verificationRequired {
				continue
			}

			var verifierReport *VerificationResult
			if artifactReport, verifierReport, err = e.verifyAndAppendResult(ctx, verifier, artifactReport, subjectDesc, artifact, repo); err != nil {
				return nil, err
			}

			if err = e.PolicyEnforcer.EvaluateResult(ctx, artifactReport, verifierReport); err != nil {
				return nil, fmt.Errorf("failed to evaluate verifier report: %w", err)
			}
		} else {
			var err error
			if artifactReport, _, err = e.verifyAndAppendResult(ctx, verifier, artifactReport, subjectDesc, artifact, repo); err != nil {
				return nil, err
			}
		}
	}

	return artifactReport, nil
}

func (e *Executor) verifyAndAppendResult(ctx context.Context, verifier Verifier, artifactReport *ValidationReport, subject, artifact ocispec.Descriptor, repo string) (*ValidationReport, *VerificationResult, error) {
	// Verify the subject artifact against the referrer artifact.
	verifierReport, err := verifier.Verify(ctx, &VerifyOptions{
		Store:              e.Store,
		Repository:         repo,
		SubjectDescriptor:  subject,
		ArtifactDescriptor: artifact,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to verify artifact %s@%s with verifier %s: %w", repo, subject.Digest, verifier.Name(), err)
	}

	artifactReport.Results = append(artifactReport.Results, verifierReport)
	return artifactReport, verifierReport, nil
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
