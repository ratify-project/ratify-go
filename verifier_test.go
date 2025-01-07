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

import "testing"

const test = "test"

func createVerifier(_ CreateVerifierOptions) (Verifier, error) {
	return nil, nil
}

func TestRegisterVerifier_EmptyType_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected to panic")
		}
	}()
	RegisterVerifier("", createVerifier)
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
		registeredVerifiers = make(map[string]func(CreateVerifierOptions) (Verifier, error))
	}()
	RegisterVerifier(test, createVerifier)
	RegisterVerifier(test, createVerifier)
}

func TestCreateVerifier(t *testing.T) {
	RegisterVerifier(test, createVerifier)
	defer func() {
		registeredVerifiers = make(map[string]func(CreateVerifierOptions) (Verifier, error))
	}()

	tests := []struct {
		name        string
		opts        CreateVerifierOptions
		expectedErr bool
	}{
		{
			name:        "no type provided",
			opts:        CreateVerifierOptions{},
			expectedErr: true,
		},
		{
			name: "non-registered type",
			opts: CreateVerifierOptions{
				Name: test,
				Type: "non-registered",
			},
			expectedErr: true,
		},
		{
			name: "registered type",
			opts: CreateVerifierOptions{
				Name: test,
				Type: test,
			},
			expectedErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := CreateVerifier(test.opts)
			if test.expectedErr != (err != nil) {
				t.Errorf("Expected error: %v, got: %v", test.expectedErr, err)
			}
		})
	}
}
