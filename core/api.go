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

package core

import (
	"context"

	"github.com/ratify-project/ratify-go/plugin/policyenforcer"
)

// VerifyParameters describes the artifact validation parameters
type VerifyParameters struct {
	Subject        string   `json:"subjectReference"`
	ReferenceTypes []string `json:"referenceTypes,omitempty"`
}

// Executor is an interface that defines methods to validate an artifact
type Executor interface {
	// ValidateArtifact returns the result of verifying an artifact
	ValidateArtifact(ctx context.Context, verifyParameters VerifyParameters) (policyenforcer.ValidationResult, error)
}
