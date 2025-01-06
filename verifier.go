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

// RegisteredVerifiers saves the registered verifier factories.
var RegisteredVerifiers = make(map[string]func(config VerifierConfig) (Verifier, error))

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

// VerifierResult defines the verification result that a verifier plugin must return.
type VerifierResult struct {
	// IsSuccess indicates whether the verification was successful or not. Required.
	IsSuccess bool `json:"isSuccess"`
	// Message is a human-readable message that describes the verification result. Required.
	Message string `json:"message"`
	// VerifierName is the name of the verifier that produced this result. Required.
	VerifierName string `json:"verifierName"`
	// VerifierType is the type of the verifier that produced this result. Required.
	VerifierType string `json:"verifierType"`
	// ErrorReason is a machine-readable reason for the verification failure. Optional.
	ErrorReason string `json:"errorReason,omitempty"`
	// Remediation is a machine-readable remediation for the verification failure. Optional.
	Remediation string `json:"remediation,omitempty"`
	// Extensions is additional information that can be used to provide more
	// context about the verification result. Optional.
	Extensions interface{} `json:"extensions,omitempty"`
}

// RegisterVerifier registers a verifier factory to the system.
func RegisterVerifier(verifierType string, factory func(config VerifierConfig) (Verifier, error)) {
	if factory == nil {
		panic("verifier factory cannot be nil")
	}
	if _, registered := RegisteredVerifiers[verifierType]; registered {
		panic(fmt.Sprintf("verifier factory named %s already registered", verifierType))
	}
	RegisteredVerifiers[verifierType] = factory
}

// CreateVerifier creates a verifier instance if it belongs to a registered type.
func CreateVerifier(config VerifierConfig) (Verifier, error) {
	if config.Name == "" || config.Type == "" {
		return nil, fmt.Errorf("name or type is not provided in the verifier config")
	}
	verifierFactory, ok := RegisteredVerifiers[config.Type]
	if ok {
		return verifierFactory(config)
	}
	return nil, fmt.Errorf("verifier factory of type %s is not registered", config.Type)
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
