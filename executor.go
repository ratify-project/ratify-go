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
	"sync"

	"golang.org/x/sync/errgroup"
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

// NewExecutorOptions represents the options to create an executor.
type NewExecutorOptions struct {
	WorkerNumber int
}

// Executor is defined to validate artifacts.
type Executor struct {
	// Executor could fetch supply chain contents from multiple stores.
	stores []Store
	// Executor should have only one policy enforcer to evalute reports.
	policyEnforcer PolicyEnforcer
	// verifier wraps multiple verifiers to validate artifacts concurrently.
	verifier artifactVerifier
	// tagResolver resolves the artifact reference to a descriptor from
	// configured stores.
	tagResolver resolver
	opts        *NewExecutorOptions
}

const (
	// defaultWorkerNumber is the default number of workers to validate
	// artifacts. It could be adjusted when creating an executor.
	defaultWorkerNumber = 4

	// maxTaskQueueSize is the maximum size of the task queue. If an artifact
	// has too many referrers, executor may stop the validation and return an
	// error.
	maxTaskQueueSize = 100
)

// NewExecutor creates a new executor instance.
func NewExecutor(opts NewExecutorOptions, stores []Store, verifiers []Verifier, policyEnforcer PolicyEnforcer) *Executor {
	if opts.WorkerNumber == 0 {
		opts.WorkerNumber = defaultWorkerNumber
	}

	return &Executor{
		stores:         stores,
		policyEnforcer: policyEnforcer,
		verifier: &concurrentVerifier{
			Verifiers: verifiers,
		},
		tagResolver: &tagResolver{
			Stores: stores,
		},
		opts: &NewExecutorOptions{
			WorkerNumber: opts.WorkerNumber,
		},
	}
}

// ValidateArtifact returns the result of verifying an artifact
func (e *Executor) ValidateArtifact(ctx context.Context, opts ValidateArtifactOptions) (*ValidationResult, error) {
	aggregatedVerifierReports, err := e.aggregateVerifierReports(ctx, opts)
	if err != nil {
		return nil, err
	}

	if e.policyEnforcer == nil {
		return &ValidationResult{Succeeded: false, ArtifactReports: aggregatedVerifierReports}, nil
	}

	decision := e.policyEnforcer.Evaluate(ctx, aggregatedVerifierReports)
	return &ValidationResult{Succeeded: decision, ArtifactReports: aggregatedVerifierReports}, nil
}

// aggregateVerifierReports generates and aggregates all verifier reports.
func (e *Executor) aggregateVerifierReports(ctx context.Context, opts ValidateArtifactOptions) ([]*ValidationReport, error) {
	var aggregatedVerifierReports []*ValidationReport
	var taskWg sync.WaitGroup
	eg, errCtx := errgroup.WithContext(ctx)
	taskQueue := make(chan *task, maxTaskQueueSize)

	for i := 0; i < e.opts.WorkerNumber; i++ {
		eg.Go(func() error {
			return e.newValidatingWorker(errCtx, taskQueue, opts.ReferenceTypes, &aggregatedVerifierReports, &taskWg)
		})
	}

	taskWg.Add(1)
	// Enqueue the root task to start the validation process.
	taskQueue <- newTask(opts.Subject, nil)

	// Spin up a goroutine to wait for all produced tasks to be consumed.
	// There are 2 ways to exit this goroutine:
	// 1. All tasks are consumed successfully.
	// 2. An error occurs or the context is canceled. Then the main goroutine
	// will keep invoking taskWg.Done() until the WaitGroup counter goes back to
	// 0 to ensure taskWg.Wait() is unblocked.
	go func() {
		taskWg.Wait()
		// Close the taskQueue channel so that the worker goroutines can exit
		// along with nil error.
		defer close(taskQueue)
	}()

	// There are 2 ways to exit the Wait() function:
	// 1. All tasks are consumed successfully and the taskQueue channel is
	// closed to signal each worker goroutine to return nil error.
	// 2. An error occurs or the context is canceled.
	err := eg.Wait()
	// Ensure pending items are processed.
	for range taskQueue {
		taskWg.Done()
	}
	if err != nil {
		return nil, err
	}

	return aggregatedVerifierReports, nil
}

// newValidatingWorker creates a new worker to validate artifacts continuously.
// A worker keeps consuming tasks from the taskQueue and producing new tasks to
// the taskQueue if necessary.
func (e *Executor) newValidatingWorker(ctx context.Context, taskQueue chan *task, referenceTypes []string, aggregatedVerifierReports *[]*ValidationReport, taskWg *sync.WaitGroup) error {
	for current := range taskQueue {
		if err := e.validateArtifact(ctx, current.artifact, current.subjectReport, aggregatedVerifierReports, referenceTypes, taskQueue, taskWg); err != nil {
			return err
		}
	}
	return nil
}

// validateArtifact will iterate over all configured stores and validate the
// artifact in the matching store.
func (e *Executor) validateArtifact(ctx context.Context, artifactRef string, subjectReport *threadSafeReport, aggregatedVerifierReports *[]*ValidationReport, artifactTypes []string, taskQueue chan *task, taskWg *sync.WaitGroup) error {
	// Ensure to decrease the WaitGroup counter when current task is processed.
	defer taskWg.Done()

	ref, err := registry.ParseReference(artifactRef)
	if err != nil {
		return fmt.Errorf("failed to parse artifact reference %s: %w", artifactRef, err)
	}

	artifactDesc, err := e.tagResolver.Resolve(ctx, ref.String())
	if err != nil {
		return fmt.Errorf("failed to resolve artifact reference %s: %w", ref.Reference, err)
	}
	ref.Reference = artifactDesc.Digest.String()

	eg, errCtx := errgroup.WithContext(ctx)
	// We need to verify the artifact against its required referrer artifacts.
	// artifactReports is used to store the validation reports of those
	// referrer artifacts.
	artifactReports := make([]*ValidationReport, 0)
	for _, store := range e.stores {
		store := store
		eg.Go(func() error {
			return e.validatePerStore(errCtx, store, ref, taskQueue, artifactTypes, &artifactReports, taskWg)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	// If the current task is the root task, add the artifactReports to the
	// aggregatedVerifierReports. Otherwise, they are just artifactReports of
	// the subject artifact.
	if subjectReport != nil {
		subjectReport.addArtifactReports(artifactReports)
	} else {
		*aggregatedVerifierReports = append(*aggregatedVerifierReports, artifactReports...)
	}

	return nil
}

// validatePerStore validates the subject artifact against all referrers in the
// store and produces new tasks for each referrer.
func (e *Executor) validatePerStore(ctx context.Context, store Store, subject registry.Reference, taskQueue chan *task, referenceTypes []string, artifactReports *[]*ValidationReport, taskWg *sync.WaitGroup) error {
	referrers, err := store.ListReferrers(ctx, subject.String(), referenceTypes, nil)
	if err != nil {
		return err
	}

	wg, errCtx := errgroup.WithContext(ctx)
	for _, referrer := range referrers {
		referrer := referrer
		wg.Go(func() error {
			artifactReport := &ValidationReport{}
			artifactReport.Results, err = e.verifier.Verify(errCtx, store, subject.String(), referrer)
			if err != nil {
				return err
			}
			*artifactReports = append(*artifactReports, artifactReport)
			referrerRef := registry.Reference{
				Registry:   subject.Registry,
				Repository: subject.Repository,
				Reference:  referrer.Digest.String(),
			}

			// Enqueue the current referrer as a new task. If the task queue is
			// full, return an error to stop the whole process. If it just keeps
			// waiting instead of returning an error, it may cause a deadlock.
			select {
			case taskQueue <- newTask(referrerRef.String(), artifactReport):
				taskWg.Add(1)
			default:
				return errors.New("task queue is full")
			}
			return nil
		})
	}
	return wg.Wait()
}
