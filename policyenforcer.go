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

// registeredPolicyEnforcers saves the registered policy enforcer factories.
var registeredPolicyEnforcers map[string]func(config CreatePolicyEnforcerOption) (PolicyEnforcer, error)

// ReferrerValidationReport describes the results of verifying an associated artifact
// and its nested artifacts by available verifiers.
type ReferrerValidationReport struct {
	// Subject is the subject artifact of the current referrer being verified. Required.
	Subject string

	// Descriptor is the descriptor of the referrer artifact being verified. Required.
	Descriptor ocispec.Descriptor

	// VerifierReports is reports of verifying current referrer artifact by matching verifiers. Required.
	VerifierReports []*VerifierResult

	// ReferrerReports is reports of verifying referrer artifacts of current referrer. Required.
	ReferrerReports []*ReferrerValidationReport
}

// PolicyEnforcer is an interface with methods that make policy decisions.
type PolicyEnforcer interface {
	// EvaluateValidationReports determines the final outcome of validation that
	// is constructed using the results from individual verifications.
	EvaluateValidationReports(ctx context.Context, validationReports []*ReferrerValidationReport) bool
}

// CreatePolicyEnforcerOption represents the options to create a policy enforcer plugin.
type CreatePolicyEnforcerOption struct {
	// Name is unique identifier of a policy enforcer instance. Required.
	Name string
	// Type represents a specific implementation of policy enforcer. Required.
	// Note: there could be multiple policy enforcers of the same type with different names.
	Type string
	// Parameters of the policy enforcer. Optional.
	Parameters any
}

// RegisterPolicyEnforcer registers a policy enforcer factory to the system.
func RegisterPolicyEnforcer(policyEnforcerType string, factory func(opts CreatePolicyEnforcerOption) (PolicyEnforcer, error)) {
	if policyEnforcerType == "" {
		panic("policy enforcer type cannot be empty")
	}
	if factory == nil {
		panic("policy enforcer factory cannot be nil")
	}
	if registeredPolicyEnforcers == nil {
		registeredPolicyEnforcers = make(map[string]func(opts CreatePolicyEnforcerOption) (PolicyEnforcer, error))
	}
	if _, registered := registeredPolicyEnforcers[policyEnforcerType]; registered {
		panic(fmt.Sprintf("policy enforcer factory type %s already registered", policyEnforcerType))
	}
	registeredPolicyEnforcers[policyEnforcerType] = factory
}

// CreatePolicyEnforcer creates a policy enforcer instance if it belongs to a registered type.
func CreatePolicyEnforcer(opts CreatePolicyEnforcerOption) (PolicyEnforcer, error) {
	if opts.Name == "" || opts.Type == "" {
		return nil, fmt.Errorf("name or type is not provided in the policy enforcer config")
	}
	policyEnforcerFactory, ok := registeredPolicyEnforcers[opts.Type]
	if ok {
		return policyEnforcerFactory(opts)
	}
	return nil, fmt.Errorf("policy enforcer factory of type %s is not registered", opts.Type)
}
