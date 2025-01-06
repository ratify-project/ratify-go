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

func createStore(config StoreConfig) (ReferrerStore, error) {
	return nil, nil
}

func TestRegisterStore_NilFactory_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected to panic")
		}
	}()
	RegisterStore(test, nil)
}

func TestRegisterStore_DuplicateFactory_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected to panic")
		}
		RegisteredStores = make(map[string]func(config StoreConfig) (ReferrerStore, error))
	}()
	RegisterStore(test, createStore)
	RegisterStore(test, createStore)
}

func TestCreateStore(t *testing.T) {
	RegisterStore(test, createStore)
	defer func() {
		RegisteredStores = make(map[string]func(config StoreConfig) (ReferrerStore, error))
	}()

	tests := []struct {
		name        string
		config      StoreConfig
		expectedErr bool
	}{
		{
			name:        "no type provided",
			config:      StoreConfig{},
			expectedErr: true,
		},
		{
			name: "non-registered type",
			config: StoreConfig{
				Name: test,
				Type: "non-registered",
			},
			expectedErr: true,
		},
		{
			name: "registered type",
			config: StoreConfig{
				Name: test,
				Type: test,
			},
			expectedErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := CreateStore(test.config)
			if test.expectedErr != (err != nil) {
				t.Errorf("Expected error: %v, got: %v", test.expectedErr, err)
			}
		})
	}
}
