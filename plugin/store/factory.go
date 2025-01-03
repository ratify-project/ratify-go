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

package store

import (
	"fmt"
)

// RegisteredStores saves the registered store factories.
var RegisteredStores = make(map[string]StoreFactory)

// StoreConfig represents the configuration of a store.
type StoreConfig struct {
	// Name is unique identifier of the store. Required.
	Name string `json:"name"`
	// Type of the store. Required.
	// Note: there could be multiple stores of the same type with different names.
	Type string `json:"type"`
	// Parameters of the store. Optional.
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// StoreFactory is an interface that defines create method to create a store
// instance of a specific type. Each Store implementation should have a corresponding
// factory that implements this interface.
type StoreFactory interface {
	Create(config StoreConfig) (ReferrerStore, error)
}

// Register registers a store factory to the system.
func Register(name string, factory StoreFactory) {
	if factory == nil {
		panic("store factory cannot be nil")
	}
	if _, registered := RegisteredStores[name]; registered {
		panic(fmt.Sprintf("store factory named %s already registered", name))
	}
	RegisteredStores[name] = factory
}

// CreateStore creates a store instance if it belongs to a registered type.
func CreateStore(config StoreConfig) (ReferrerStore, error) {
	if config.Name == "" || config.Type == "" {
		return nil, fmt.Errorf("name or type is not provided in the store config")
	}
	storeFactory, ok := RegisteredStores[config.Type]
	if ok {
		return storeFactory.Create(config)
	}
	return nil, fmt.Errorf("store factory of type %s is not registered", config.Type)
}
