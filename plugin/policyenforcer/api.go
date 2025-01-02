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

package policyenforcer

import (
	"context"

	"github.com/ratify-project/ratify-go/plugin/verifier"
)

// ValidationResult aggregates verifier reports and the final verification
// result evaluated by the policy enforcer.
type ValidationResult struct {
	IsSuccess       bool                        `json:"isSuccess,omitempty"`
	ArtifactReports []*ArtifactValidationReport `json:"artifactReports"`
}

// ArtifactValidationReport describes the results of verifying an artifact and its
// nested artifacts by available verifiers.
type ArtifactValidationReport struct {
	Subject               string                      `json:"subject"`
	ReferenceDigest       string                      `json:"referenceDigest"`
	ArtifactType          string                      `json:"artifactType"`
	VerifierReports       []verifier.VerifierResult   `json:"verifierReports"`
	NestedArtifactReports []*ArtifactValidationReport `json:"nestedReports"`
}

// PolicyEnforcer is an interface with methods that represents policy decisions.
type PolicyEnforcer interface {
	// ErrorToVerifyResult converts an error to a properly formatted verify result
	ErrorToVerifyResult(ctx context.Context, subjectRefString string, verifyError error) ValidationResult
	// EvaluateVerifierReport determines the final outcome of verification that is constructed using the results from
	// individual verifications
	EvaluateVerifierReport(ctx context.Context, verifierReports []*ArtifactValidationReport) bool
}
