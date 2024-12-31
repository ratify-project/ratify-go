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
	"fmt"
)

// RegisteredPolicyEnforcers saves the registered policy enforcer factories.
var RegisteredPolicyEnforcers = make(map[string]PolicyEnforcerFactory)

// PolicyEnforcerConfig represents the configuration of a policy plugin
type PolicyEnforcerConfig struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Parameters map[string]interface{} `json:"parameters"`
}

// PolicyEnforcerFactory is an interface that defines create method to create a
// policy enforcer instance of a specific type. Each PolicyEnforcer implementation
// should have a corresponding factory that implements this interface.
type PolicyEnforcerFactory interface {
	Create(config PolicyEnforcerConfig) (PolicyEnforcer, error)
}

// Register registers a policy enforcer factory to the system.
func Register(name string, factory PolicyEnforcerFactory) {
	if factory == nil {
		panic("policy enforcer factory cannot be nil")
	}
	if _, registered := RegisteredPolicyEnforcers[name]; registered {
		panic(fmt.Sprintf("policy enforcer factory named %s already registered", name))
	}
	RegisteredPolicyEnforcers[name] = factory
}

// CreatePolicyEnforcer creates a policy enforcer instance if it belongs to a registered type.
func CreatePolicyEnforcer(config PolicyEnforcerConfig) (PolicyEnforcer, error) {
	if config.Name == "" || config.Type == "" {
		return nil, fmt.Errorf("name or type is not provided in the policy enforcer config")
	}
	policyEnforcerFactory, ok := RegisteredPolicyEnforcers[config.Type]
	if ok {
		return policyEnforcerFactory.Create(config)
	}
	return nil, fmt.Errorf("policy enforcer factory of type %s is not registered", config.Type)
}
