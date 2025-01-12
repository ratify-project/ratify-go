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
	// Executor could configure multiple stores to fetch supply chain content.
	// But each validating artifact should be restricted to a single store.
	// Required.
	Stores []Store

	// Executor could use multiple verifiers to validate artifacts. Required.
	Verifiers []Verifier

	// Executor should have at most one policy enforcer to evalute reports. If
	// not set, the validation result will be returned without evaluation.
	// Optional.
	PolicyEnforcer PolicyEnforcer
}

// ValidateArtifact returns the result of verifying an artifact.
func (e *Executor) ValidateArtifact(ctx context.Context, opts ValidateArtifactOptions) (*ValidationResult, error) {
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
	var err error

	// TODO: Implement a worker pool to validate artifacts concurrently.
	// TODO: Enforce check on the stack size.
	taskStack := make([]*task, 0)

	// Iterate over all stores to create tasks for each store so that further
	// referrer artifacts will be fetched from the same store.
	for _, store := range e.Stores {
		taskStack = append(taskStack, &task{
			store:    store,
			artifact: opts.Subject,
		})
	}

	for len(taskStack) > 0 {
		task := taskStack[len(taskStack)-1]
		taskStack = taskStack[:len(taskStack)-1]

		taskStack, aggregatedVerifierReports, err = e.processTask(ctx, task, taskStack, aggregatedVerifierReports, opts.ReferenceTypes)
		if err != nil {
			return nil, fmt.Errorf("failed to validate artifact %s: %w", task.artifact, err)
		}
	}

	return aggregatedVerifierReports, nil
}

// processTask processes the task and returns the updated stack and aggregated
// verifier reports.
func (e *Executor) processTask(ctx context.Context, task *task, stack []*task, aggregatedVerifierReports []*ValidationReport, referenceTypes []string) ([]*task, []*ValidationReport, error) {
	artifactRef := task.artifact
	ref, err := registry.ParseReference(artifactRef)
	if err != nil {
		return stack, nil, fmt.Errorf("failed to parse artifact reference %s: %w", artifactRef, err)
	}

	artifactDesc, err := task.store.Resolve(ctx, ref.String())
	if err != nil {
		return stack, nil, fmt.Errorf("failed to resolve artifact reference %s: %w", ref.Reference, err)
	}
	ref.Reference = artifactDesc.Digest.String()

	stack, artifactReports, err := e.verifyAgainstReferrers(ctx, task.store, ref, stack, referenceTypes)
	if err != nil {
		return stack, nil, err
	}

	// If the current task is the root task, add the artifactReports to the
	// aggregatedVerifierReports. Otherwise, they are just artifactReports of
	// the subject artifact.
	if task.subjectReport != nil {
		task.subjectReport.ArtifactReports = append(task.subjectReport.ArtifactReports, artifactReports...)
	} else {
		aggregatedVerifierReports = append(aggregatedVerifierReports, artifactReports...)
	}
	return stack, aggregatedVerifierReports, nil
}

// verifyAgainstReferrers verifies the subject artifact against all referrers in
// the store and produces new tasks for each referrer.
func (e *Executor) verifyAgainstReferrers(ctx context.Context, store Store, subject registry.Reference, stack []*task, referenceTypes []string) ([]*task, []*ValidationReport, error) {
	referrers, err := store.ListReferrers(ctx, subject.String(), referenceTypes, nil)
	if err != nil {
		return stack, nil, fmt.Errorf("failed to list referrers for artifact %s: %w", subject.String(), err)
	}

	// We need to verify the artifact against its required referrer artifacts.
	// artifactReports is used to store the validation reports of those
	// referrer artifacts.
	artifactReports := make([]*ValidationReport, 0)
	for _, referrer := range referrers {
		artifactReport := &ValidationReport{}
		artifactReport.Results, err = e.verifyArtifact(ctx, store, subject.String(), referrer)
		if err != nil {
			return stack, nil, err
		}
		artifactReports = append(artifactReports, artifactReport)
		referrerRef := registry.Reference{
			Registry:   subject.Registry,
			Repository: subject.Repository,
			Reference:  referrer.Digest.String(),
		}

		stack = append(stack, &task{
			store:         store,
			artifact:      referrerRef.String(),
			subjectReport: artifactReport,
		})
	}
	return stack, artifactReports, nil
}

// verifyArtifact verifies the artifact by all configured verifiers and returns
// error if any of the verifier fails.
func (e *Executor) verifyArtifact(ctx context.Context, store Store, subject string, artifact ocispec.Descriptor) ([]*VerificationResult, error) {
	verifierReports := make([]*VerificationResult, 0)

	for _, verifier := range e.Verifiers {
		if !verifier.Verifiable(artifact) {
			continue
		}
		verifier := verifier
		var verifierReport *VerificationResult
		verifierReport, err := verifier.Verify(ctx, store, subject, artifact)
		if err != nil {
			return nil, fmt.Errorf("failed to verify artifact %s with verifier %s: %w", subject, verifier.Name(), err)
		}

		verifierReports = append(verifierReports, verifierReport)
	}

	return verifierReports, nil
}
