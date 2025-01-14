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

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// registeredStores saves the registered store factories.
var registeredStores map[string]func(CreateStoreOptions) (Store, error)

// Store is an interface that defines methods to query the graph of supply chain
// content including its related content
type Store interface {
	// Name is the name of the store.
	Name() string

	// Resolve resolves to a descriptor for the given artifact reference.
	Resolve(ctx context.Context, ref string) (ocispec.Descriptor, error)

	// ListReferrers returns the immediate set of supply chain artifacts for the
	// given subject reference.
	// Note: This API supports pagination. fn should be set to handle the
	//       underlying pagination.
	ListReferrers(ctx context.Context, ref string, artifactTypes []string, fn func(referrers []ocispec.Descriptor) error) error

	// FetchBlobContent returns the blob by the given reference.
	// WARNING: This API is intended to use for small objects like signatures,
	//          SBoMs.
	FetchBlobContent(ctx context.Context, repo string, desc ocispec.Descriptor) ([]byte, error)

	// FetchImageManifest returns the referenced image manifest as given by the
	// descriptor.
	FetchImageManifest(ctx context.Context, repo string, desc ocispec.Descriptor) (*ocispec.Manifest, error)
}

// CreateStoreOptions represents the options to create a store.
type CreateStoreOptions struct {
	// Name is unique identifier of a store instance. Required.
	Name string

	// Type represents a specific implementation of stores. Required.
	// Note: there could be multiple stores of the same type with different 
	//       names.
	Type string

	// Parameters of the store. Optional.
	Parameters any
}

// RegisterStore registers a store factory to the system.
func RegisterStore(storeType string, create func(CreateStoreOptions) (Store, error)) {
	if storeType == "" {
		panic("store type cannot be empty")
	}
	if create == nil {
		panic("store factory cannot be nil")
	}
	if registeredStores == nil {
		registeredStores = make(map[string]func(CreateStoreOptions) (Store, error))
	}
	if _, registered := registeredStores[storeType]; registered {
		panic(fmt.Sprintf("store factory type %s already registered", storeType))
	}
	registeredStores[storeType] = create
}

// CreateStore creates a store instance if it belongs to a registered type.
func CreateStore(opts CreateStoreOptions) (Store, error) {
	if opts.Name == "" || opts.Type == "" {
		return nil, fmt.Errorf("name or type is not provided in the store options")
	}
	storeFactory, ok := registeredStores[opts.Type]
	if !ok {
		return nil, fmt.Errorf("store factory of type %s is not registered", opts.Type)
	}
	return storeFactory(opts)
}
