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
	"sync"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sync/errgroup"
)

// artifactVerifier is an interface defines the method to verify an artifact and
// return the verification results.
type artifactVerifier interface {
	Verify(ctx context.Context, store Store, subject string, artifact ocispec.Descriptor) ([]*VerificationResult, error)
}

// concurrentVerifier implements the artifactVerifier interface to verify an
// artifact concurrently using multiple verifiers.
type concurrentVerifier struct {
	Verifiers []Verifier
}

// Verify verifies the artifact using multiple verifiers concurrently and
// returns error if any of the verifier fails.
func (v *concurrentVerifier) Verify(ctx context.Context, store Store, subject string, artifact ocispec.Descriptor) ([]*VerificationResult, error) {
	verifierReports := make([]*VerificationResult, 0)
	var mu sync.Mutex
	eg, errCtx := errgroup.WithContext(ctx)

	for _, verifier := range v.Verifiers {
		if !verifier.Verifiable(artifact) {
			continue
		}
		verifier := verifier
		eg.Go(func() error {
			var verifierReport *VerificationResult
			verifierReport, err := verifier.Verify(errCtx, store, subject, artifact)
			verifierReport.Err = err

			mu.Lock()
			verifierReports = append(verifierReports, verifierReport)
			mu.Unlock()

			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return verifierReports, nil
}
