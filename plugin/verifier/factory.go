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

import "fmt"

// RegisteredVerifiers saves the registered verifier factories.
var RegisteredVerifiers = make(map[string]VerifierFactory)

// VerifierConfig represents the configuration of a verifier.
type VerifierConfig struct {
	// Name is the unique identifier of the verifier. Required.
	Name string `json:"name"`
	// Type is the type of the verifier. Note: there could be multiple verifiers of the same type with different names. Required.
	Type string `json:"type"`
	// ArtifactTypes is the list of artifact types that the verifier can verify. Required.
	ArtifactTypes []string `json:"artifactTypes"`
	// Parameters is additional parameters of the verifier. Optional.
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// VerifierFactory is an interface that defines create method to create a verifier
// instance of a specific type. Each Verifier implementation should have a corresponding
// factory that implements this interface.
type VerifierFactory interface {
	Create(config VerifierConfig) (Verifier, error)
}

// Register registers a verifier factory to the system.
func Register(verifierType string, factory VerifierFactory) {
	if factory == nil {
		panic("verifier factory cannot be nil")
	}
	if _, registered := RegisteredVerifiers[verifierType]; registered {
		panic(fmt.Sprintf("verifier factory named %s already registered", verifierType))
	}
	RegisteredVerifiers[verifierType] = factory
}

// CreateVerifier creates a verifier instance if it belongs to a registered type.
func CreateVerifier(config VerifierConfig) (Verifier, error) {
	if config.Name == "" || config.Type == "" {
		return nil, fmt.Errorf("name or type is not provided in the verifier config")
	}
	verifierFactory, ok := RegisteredVerifiers[config.Type]
	if ok {
		return verifierFactory.Create(config)
	}
	return nil, fmt.Errorf("verifier factory of type %s is not registered", config.Type)
}
