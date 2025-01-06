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
	"context"
	"fmt"

	"github.com/opencontainers/go-digest"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/ratify-project/ratify-go/internal/common"
)

// RegisteredStores saves the registered store factories.
var RegisteredStores = make(map[string]func(config StoreConfig) (ReferrerStore, error))

// ListReferrersResult represents a paginated result of ListReferrers API.
type ListReferrersResult struct {
	// Referrers is the list of referrers in the current page.
	Referrers []oci.Descriptor
	// NextToken is the token to get the next page of results.
	NextToken string
}

// ReferrerStore is an interface that defines methods to query the graph of supply chain content including its related content
type ReferrerStore interface {
	// Name is the name of the store
	Name() string

	// ListReferrers returns the immediate set of supply chain artifacts for the given subject
	// represented as artifact manifests.
	// Note: This API supports pagination. The nextToken is used to get the next page of results.
	// The nextToken is supposed to be empty if there are no more results.
	ListReferrers(ctx context.Context, subjectReference common.Reference, artifactTypes []string, nextToken string) (ListReferrersResult, error)

	// GetBlobContent returns the blob with the given digest.
	// WARNING: This API is intended to use for small objects like signatures, SBoMs.
	GetBlobContent(ctx context.Context, subjectReference common.Reference, digest digest.Digest) ([]byte, error)

	// GetReferenceManifest returns the reference artifact manifest as given by the descriptor.
	GetReferenceManifest(ctx context.Context, subjectReference common.Reference, referenceDesc oci.Descriptor) (oci.Manifest, error)

	// GetSubjectDescriptor returns the descriptor for the given subject.
	GetSubjectDescriptor(ctx context.Context, subjectReference common.Reference) (oci.Descriptor, error)
}

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

// RegisterStore registers a store factory to the system.
func RegisterStore(name string, factory func(config StoreConfig) (ReferrerStore, error)) {
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
		return storeFactory(config)
	}
	return nil, fmt.Errorf("store factory of type %s is not registered", config.Type)
}
