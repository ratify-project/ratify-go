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

	oci "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/ratify-project/ratify-go/errors"
	"github.com/ratify-project/ratify-go/internal/common"
)

// registeredVerifiers saves the registered verifier factories.
var registeredVerifiers = make(map[string]func(config VerifierConfig) (Verifier, error))

// VerifierResult defines the verification result that a verifier plugin must return.
type VerifierResult struct {
	// Err is the error that occurred when the verification failed.
	// If the verification is successful, this field should be nil.
	Err error

	// Description describes the verification result if needed.
	Description string

	// Verifier refers to the verifier that generated the result.
	Verifier Verifier

	// Extensions is additional information that can be used to provide more
	// context about the verification result.
	Extensions any
}

// Verifier is an interface that defines methods to verify an artifact associated
// with a subject.
type Verifier interface {
	// Name returns the name of the verifier
	Name() string

	// Type returns the type name of the verifier
	Type() string

	// CanVerify returns if the verifier can verify the given reference
	CanVerify(ctx context.Context, referrerDescriptor oci.Descriptor) bool

	// Verify verifies the given reference of a subject and returns the verification result
	Verify(ctx context.Context,
		subjectReference common.Reference,
		referrerDescriptor oci.Descriptor,
		referrerStore ReferrerStore) (VerifierResult, error)
}

// VerifierConfig represents the configuration of a verifier.
type VerifierConfig struct {
	// Name is the unique identifier of the verifier. Required.
	Name string `json:"name"`
	// Type is the type of the verifier. Note: there could be multiple verifiers of the same type with different names. Required.
	Type string `json:"type"`
	// ArtifactTypes is the list of artifact types that the verifier can verify. Required.
	ArtifactTypes []string `json:"artifactTypes"`
	// Parameters is additional parameters of the verifier. Optional.
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// CreateVerifier creates a verifier instance if it belongs to a registered type.
func CreateVerifier(config VerifierConfig) (Verifier, error) {
	if config.Name == "" || config.Type == "" {
		return nil, fmt.Errorf("name or type is not provided in the verifier config")
	}
	verifierFactory, ok := registeredVerifiers[config.Type]
	if ok {
		return verifierFactory(config)
	}
	return nil, fmt.Errorf("verifier factory of type %s is not registered", config.Type)
}

// RegisterVerifier registers a verifier factory to the system.
func RegisterVerifier(verifierType string, factory func(config VerifierConfig) (Verifier, error)) {
	if factory == nil {
		panic("verifier factory cannot be nil")
	}
	if _, registered := registeredVerifiers[verifierType]; registered {
		panic(fmt.Sprintf("verifier factory named %s already registered", verifierType))
	}
	registeredVerifiers[verifierType] = factory
}

// NewVerifierResult creates a new VerifierResult object with the given parameters.
func NewVerifierResult(verifierName, verifierType, message string, isSuccess bool, err *errors.Error, extensions interface{}) VerifierResult {
	var errorReason, remediation string
	if err != nil {
		if err.GetDetail() != "" {
			message = err.GetDetail()
		}
		errorReason = err.GetErrorReason()
		remediation = err.GetRemediation()
	}
	return VerifierResult{
		IsSuccess:    isSuccess,
		VerifierName: verifierName,
		VerifierType: verifierType,
		Message:      message,
		ErrorReason:  errorReason,
		Remediation:  remediation,
		Extensions:   extensions,
	}
}
