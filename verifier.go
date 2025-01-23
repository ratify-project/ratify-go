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

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Verifier is an interface that defines methods to verify an artifact
// associated with a subject.
type Verifier interface {
	// Name returns the name of the verifier.
	Name() string

	// Type returns the type name of the verifier.
	Type() string

	// Verifiable returns if the verifier can verify against the given artifact.
	Verifiable(artifact ocispec.Descriptor) bool

	// Verify verifies the subject in the store against the artifact and
	// returns the verification result.
	Verify(ctx context.Context, opts *VerifyOptions) (*VerificationResult, error)
}

// VerifyOptions represents the options to verify a subject against an artifact.
type VerifyOptions struct {
	// Store is the store to access the artifacts. Required.
	Store Store

	// Subject is the subject reference of the artifact being verified.
	// Required.
	Subject string

	// SubjectDescriptor is the descriptor of the subject being verified.
	// Required.
	SubjectDescriptor ocispec.Descriptor

	// ArtifactDescriptor is the descriptor of the artifact being verified
	// against. Required.
	ArtifactDescriptor ocispec.Descriptor
}

// VerificationResult defines the verification result that a verifier plugin
// must return.
type VerificationResult struct {
	// Err is the error that occurred when the verification failed.
	// If the verification is successful, this field should be nil. Optional.
	Err error

	// Description describes the verification result if needed. Optional.
	Description string

	// Verifier refers to the verifier that generated the result. Required.
	Verifier Verifier

	// Detail is additional information that can be used to provide more context
	// about the verification result. Optional.
	Detail any
}
