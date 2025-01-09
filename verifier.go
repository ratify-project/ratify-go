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
)

// registeredVerifiers saves the registered verifier factories.
var registeredVerifiers map[string]func(CreateVerifierOptions) (Verifier, error)

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
	Verify(ctx context.Context, store Store, subject string, artifact ocispec.Descriptor) (*VerificationResult, error)
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

// CreateVerifierOptions represents the options to create a verifier.
type CreateVerifierOptions struct {
	// Name is the unique identifier of a verifier instantce. Required.
	Name string

	// Type represents a specific implementation of a verifier. Required.
	// Note: there could be multiple verifiers of the same type with different
	//       names.
	Type string

	// Parameters is additional parameters of the verifier. Optional.
	Parameters any
}

// RegisterVerifier registers a verifier factory to the system.
func RegisterVerifier(verifierType string, create func(CreateVerifierOptions) (Verifier, error)) {
	if verifierType == "" {
		panic("verifier type cannot be empty")
	}
	if create == nil {
		panic("verifier factory cannot be nil")
	}
	if registeredVerifiers == nil {
		registeredVerifiers = make(map[string]func(CreateVerifierOptions) (Verifier, error))
	}
	if _, registered := registeredVerifiers[verifierType]; registered {
		panic(fmt.Sprintf("verifier factory named %s already registered", verifierType))
	}
	registeredVerifiers[verifierType] = create
}

// CreateVerifier creates a verifier instance if it belongs to a registered type.
func CreateVerifier(opts CreateVerifierOptions) (Verifier, error) {
	if opts.Name == "" || opts.Type == "" {
		return nil, fmt.Errorf("name or type is not provided in the verifier options")
	}
	verifierFactory, ok := registeredVerifiers[opts.Type]
	if !ok {
		return nil, fmt.Errorf("verifier factory of type %s is not registered", opts.Type)
	}
	return verifierFactory(opts)
}
