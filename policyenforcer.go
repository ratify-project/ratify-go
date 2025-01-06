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
)

// RegisteredPolicyEnforcers saves the registered policy enforcer factories.
var RegisteredPolicyEnforcers = make(map[string]func(config PolicyEnforcerConfig) (PolicyEnforcer, error))

// ValidationResult aggregates verifier reports and the final verification
// result evaluated by the policy enforcer.
type ValidationResult struct {
	// IsSuccess indicates whether the verification is successful. Optional.
	// This field is optional because the policy enforcer may not be required to
	// evaluate the reports.
	IsSuccess bool `json:"isSuccess,omitempty"`
	// ArtifactReports is reports of verifying associated artifacts. Required.
	ArtifactReports []*ArtifactValidationReport `json:"artifactReports"`
}

// ArtifactValidationReport describes the results of verifying an associated artifact
// and its nested artifacts by available verifiers.
type ArtifactValidationReport struct {
	// Subject is the artifact that is verified. Required.
	Subject string `json:"subject"`
	// ReferenceDigest is the digest of the artifact that is verified. Required.
	ReferenceDigest string `json:"referenceDigest"`
	// ArtifactType is the media type of the artifact that is verified. Required.
	ArtifactType string `json:"artifactType"`
	// VerifierReports is reports of verifying the artifact by matching verifiers. Required.
	VerifierReports []VerifierResult `json:"verifierReports"`
	// NestedArtifactReports is reports of verifying nested artifacts. Required.
	NestedArtifactReports []*ArtifactValidationReport `json:"nestedReports"`
}

// PolicyEnforcer is an interface with methods that represents policy decisions.
type PolicyEnforcer interface {
	// ErrorToValidationResult converts an error to a properly formatted validation result.
	ErrorToValidationResult(ctx context.Context, subjectReference string, verifyError error) ValidationResult
	// EvaluateValidationReports determines the final outcome of validation that
	// is constructed using the results from individual verifications.
	EvaluateValidationReports(ctx context.Context, validationReports []*ArtifactValidationReport) bool
}

// PolicyEnforcerConfig represents the configuration of a policy plugin
type PolicyEnforcerConfig struct {
	// Name is unique identifier of a policy enforcer. Required.
	Name string `json:"name"`
	// Type of the policy enforcer. Note: there could be multiple policy enforcers
	// of the same type with different names. Required.
	Type string `json:"type"`
	// Parameters of the policy enforcer. Optional.
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// RegisterPolicyEnforcer registers a policy enforcer factory to the system.
func RegisterPolicyEnforcer(name string, factory func(config PolicyEnforcerConfig) (PolicyEnforcer, error)) {
	if factory == nil {
		panic("policy enforcer factory cannot be nil")
	}
	if _, registered := RegisteredPolicyEnforcers[name]; registered {
		panic(fmt.Sprintf("policy enforcer factory named %s already registered", name))
	}
	RegisteredPolicyEnforcers[name] = factory
}

// CreatePolicyEnforcer creates a policy enforcer instance if it belongs to a registered type.
func CreatePolicyEnforcer(config PolicyEnforcerConfig) (PolicyEnforcer, error) {
	if config.Name == "" || config.Type == "" {
		return nil, fmt.Errorf("name or type is not provided in the policy enforcer config")
	}
	policyEnforcerFactory, ok := RegisteredPolicyEnforcers[config.Type]
	if ok {
		return policyEnforcerFactory(config)
	}
	return nil, fmt.Errorf("policy enforcer factory of type %s is not registered", config.Type)
}
