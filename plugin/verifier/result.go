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

package verifier

import "github.com/ratify-project/ratify-go/internal/errors"

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
