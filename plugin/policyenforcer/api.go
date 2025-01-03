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
	// IsSuccess indicates whether the verification is successful. Optional.
	// This field is optional because the policy enforcer may not be required to
	// evaluate the reports.
	IsSuccess bool `json:"isSuccess,omitempty"`
	// ArtifactReports is reports of verifying associated artifacts. Required.
	ArtifactReports []*ArtifactValidationReport `json:"artifactReports"`
}

// ArtifactValidationReport describes the results of verifying an artifact and its
// nested artifacts by available verifiers.
type ArtifactValidationReport struct {
	// Subject is the artifact that is verified. Required.
	Subject string `json:"subject"`
	// ReferenceDigest is the digest of the artifact that is verified. Required.
	ReferenceDigest string `json:"referenceDigest"`
	// ArtifactType is the media type of the artifact that is verified. Required.
	ArtifactType string `json:"artifactType"`
	// VerifierReports is reports of verifying the artifact by matching verifiers. Required.
	VerifierReports []verifier.VerifierResult `json:"verifierReports"`
	// NestedArtifactReports is reports of verifying nested artifacts. Required.
	NestedArtifactReports []*ArtifactValidationReport `json:"nestedReports"`
}

// PolicyEnforcer is an interface with methods that represents policy decisions.
type PolicyEnforcer interface {
	// ErrorToVerifyResult converts an error to a properly formatted verify result.
	ErrorToVerifyResult(ctx context.Context, subjectRefString string, verifyError error) ValidationResult
	// EvaluateVerifierReport determines the final outcome of verification that
	// is constructed using the results from individual verifications.
	EvaluateVerifierReport(ctx context.Context, verifierReports []*ArtifactValidationReport) bool
}
