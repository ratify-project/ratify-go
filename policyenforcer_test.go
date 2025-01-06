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

func createPolicyEnforcer(config PolicyEnforcerConfig) (PolicyEnforcer, error) {
	return nil, nil
}

func TestRegisterPolicyEnforcer_NilFactory_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected to panic")
		}
	}()
	RegisterPolicyEnforcer(test, nil)
}

func TestRegisterPolicyEnforcer_DuplicateFactory_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected to panic")
		}
		RegisteredPolicyEnforcers = make(map[string]func(config PolicyEnforcerConfig) (PolicyEnforcer, error))
	}()
	RegisterPolicyEnforcer(test, createPolicyEnforcer)
	RegisterPolicyEnforcer(test, createPolicyEnforcer)
}

func TestCreatePolicyEnforcer(t *testing.T) {
	RegisterPolicyEnforcer(test, createPolicyEnforcer)
	defer func() {
		RegisteredPolicyEnforcers = make(map[string]func(config PolicyEnforcerConfig) (PolicyEnforcer, error))
	}()

	tests := []struct {
		name        string
		config      PolicyEnforcerConfig
		expectedErr bool
	}{
		{
			name:        "no type provided",
			config:      PolicyEnforcerConfig{},
			expectedErr: true,
		},
		{
			name: "non-registered type",
			config: PolicyEnforcerConfig{
				Name: test,
				Type: "non-registered",
			},
			expectedErr: true,
		},
		{
			name: "registered type",
			config: PolicyEnforcerConfig{
				Name: test,
				Type: test,
			},
			expectedErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := CreatePolicyEnforcer(test.config)
			if test.expectedErr != (err != nil) {
				t.Errorf("Expected error: %v, got: %v", test.expectedErr, err)
			}
		})
	}
}
