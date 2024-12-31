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

import (
	"testing"
)

const test = "test"

type mockFactory struct{}

func (f *mockFactory) Create(config VerifierConfig) (Verifier, error) {
	return nil, nil
}

func TestRegister_NilFactory_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected to panic")
		}
	}()
	Register(test, nil)
}

func TestRegister_DuplicateFactory_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected to panic")
		}
		RegisteredVerifiers = make(map[string]VerifierFactory)
	}()
	Register(test, &mockFactory{})
	Register(test, &mockFactory{})
}

func TestCreateVerifier(t *testing.T) {
	Register(test, &mockFactory{})
	defer func() {
		RegisteredVerifiers = make(map[string]VerifierFactory)
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
