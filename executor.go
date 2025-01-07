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
)

// ValidateArtifactOptions describes the artifact validation options.
type ValidateArtifactOptions struct {
	// SubjectArtifact is the artifact to be validated. Required.
	SubjectArtifact string
	// ReferenceTypes is a list of reference types that should be verified in
	// associated artifacts. Empty list means all artifacts should be verified.
	// Optional.
	ReferenceTypes []string
}

// ValidationResult aggregates verifier reports and the final verification
// result evaluated by the policy enforcer.
type ValidationResult struct {
	// Succeeded represents the outcome determined by the policy enforcer based on
	// the aggregated verifier reports. And if an error occurs during
	// the validation process prior to policy evaluation, it will be set to `false`.
	// When passthrough mode is enabled, the policy enforcer does not evaluate
	// the result and sets this field to `false`. In such cases, this field should be ignored.
	// Required.
	Succeeded bool
	// ArtifactReports is aggregated reports of verifying associated artifacts. Required.
	ArtifactReports []*ValidationReport
}

// Executor is defined to validate artifacts.
type Executor struct{}

// ValidateArtifact returns the result of verifying an artifact
func (e *Executor) ValidateArtifact(ctx context.Context, opts ValidateArtifactOptions) (*ValidationResult, error) {
	return nil, errors.New("not implemented")
}
