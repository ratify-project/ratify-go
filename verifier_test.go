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
	"testing"

	"github.com/ratify-project/ratify-go/errors"
)

const (
	testName = "testName"
	testType = "testType"
	testMsg  = "testMsg"
	testMsg2 = "testMsg2"
	testMsg3 = "testMsg3"
	test     = "test"
)

var (
	testErr       = errors.ErrorCodeUnknown.WithDetail(testMsg2)
	testNestedErr = errors.ErrorCodeUnknown.WithError(testErr).WithDetail(testMsg3)
)

func createVerifier(config VerifierConfig) (Verifier, error) {
	return nil, nil
}

func TestRegisterVerifier_NilFactory_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected to panic")
		}
	}()
	RegisterVerifier(test, nil)
}

func TestRegisterVerifier_DuplicateFactory_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected to panic")
		}
		registeredVerifiers = make(map[string]func(config VerifierConfig) (Verifier, error))
	}()
	RegisterVerifier(test, createVerifier)
	RegisterVerifier(test, createVerifier)
}

func TestCreateVerifier(t *testing.T) {
	RegisterVerifier(test, createVerifier)
	defer func() {
		registeredVerifiers = make(map[string]func(config VerifierConfig) (Verifier, error))
	}()

	tests := []struct {
		name        string
		config      VerifierConfig
		expectedErr bool
	}{
		{
			name:        "no type provided",
			config:      VerifierConfig{},
			expectedErr: true,
		},
		{
			name: "non-registered type",
			config: VerifierConfig{
				Name: test,
				Type: "non-registered",
			},
			expectedErr: true,
		},
		{
			name: "registered type",
			config: VerifierConfig{
				Name: test,
				Type: test,
			},
			expectedErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := CreateVerifier(test.config)
			if test.expectedErr != (err != nil) {
				t.Errorf("Expected error: %v, got: %v", test.expectedErr, err)
			}
		})
	}
}

func TestNewVerifierResult(t *testing.T) {
	tests := []struct {
		name                string
		verifierName        string
		verifierType        string
		message             string
		isSuccess           bool
		err                 *errors.Error
		expectedMessage     string
		expectedRemediation string
		expectedErrorReason string
	}{
		{
			name:                "nil error",
			verifierName:        testName,
			verifierType:        testType,
			message:             testMsg,
			isSuccess:           true,
			err:                 nil,
			expectedMessage:     testMsg,
			expectedRemediation: "",
			expectedErrorReason: "",
		},
		{
			name:                "error with detail",
			verifierName:        testName,
			verifierType:        testType,
			message:             testMsg,
			isSuccess:           true,
			err:                 &testErr,
			expectedMessage:     testMsg,
			expectedRemediation: "",
			expectedErrorReason: testMsg2,
		},
		{
			name:                "nested error",
			verifierName:        testName,
			verifierType:        testType,
			message:             testMsg,
			isSuccess:           true,
			err:                 &testNestedErr,
			expectedMessage:     testMsg3,
			expectedRemediation: "",
			expectedErrorReason: testMsg2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewVerifierResult(tt.verifierName, tt.verifierType, tt.message, tt.isSuccess, tt.err, nil)
			if result.Message != tt.expectedMessage {
				t.Errorf("Expected message to be %s, got %s", tt.expectedMessage, result.Message)
			}
			if result.Remediation != tt.expectedRemediation {
				t.Errorf("Expected remediation to be %s, got %s", tt.expectedRemediation, result.Remediation)
			}
			if result.ErrorReason != tt.expectedErrorReason {
				t.Errorf("Expected error reason to be %s, got %s", tt.expectedErrorReason, result.ErrorReason)
			}
			if result.VerifierName != tt.verifierName {
				t.Errorf("Expected verifier name to be %s, got %s", tt.verifierName, result.VerifierName)
			}
			if result.VerifierType != tt.verifierType {
				t.Errorf("Expected verifier type to be %s, got %s", tt.verifierType, result.VerifierType)
			}
			if result.IsSuccess != tt.isSuccess {
				t.Errorf("Expected isSuccess to be %t, got %t", tt.isSuccess, result.IsSuccess)
			}
		})
	}
}
