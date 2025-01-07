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
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry"
)

// registeredStores saves the registered store factories.
var registeredStores map[string]func(config CreateStorOptions) (Store, error)

// Store is an interface that defines methods to query the graph of supply chain content including its related content
type Store interface {
	// Name is the name of the store
	Name() string

	// ListReferrers returns the immediate set of supply chain artifacts for the given subject
	// represented as artifact manifests.
	// Note: This API supports pagination. fn should be set to handle the underlying pagination.
	ListReferrers(ctx context.Context, subjectReference registry.Reference, subjectDescriptor ocispec.Descriptor, artifactTypes []string, fn func(referrers []ocispec.Descriptor) error) ([]ocispec.Descriptor, error)

	// GetBlobContent returns the blob with the given digest and its subject.
	// WARNING: This API is intended to use for small objects like signatures, SBoMs.
	GetBlobContent(ctx context.Context, subjectReference string, digest digest.Digest) ([]byte, error)

	// GetReferenceManifest returns the referenced artifact manifest as given by the descriptor and its subject.
	GetReferenceManifest(ctx context.Context, subjectReference string, referenceDesc ocispec.Descriptor) (ocispec.Manifest, error)

	// GetSubjectDescriptor returns the descriptor for the given subject.
	GetSubjectDescriptor(ctx context.Context, subjectReference string) (ocispec.Descriptor, error)
}

// CreateStorOptions represents the options to create a store.
type CreateStorOptions struct {
	// Name is unique identifier of a store instance. Required.
	Name string
	// Type represents a specific implementation of stores. Required.
	// Note: there could be multiple stores of the same type with different names.
	Type string
	// Parameters of the store. Optional.
	Parameters any
}

// RegisterStore registers a store factory to the system.
func RegisterStore(storeType string, factory func(config CreateStorOptions) (Store, error)) {
	if storeType == "" {
		panic("store type cannot be empty")
	}
	if factory == nil {
		panic("store factory cannot be nil")
	}
	if registeredStores == nil {
		registeredStores = make(map[string]func(config CreateStorOptions) (Store, error))
	}
	if _, registered := registeredStores[storeType]; registered {
		panic(fmt.Sprintf("store factory type %s already registered", storeType))
	}
	registeredStores[storeType] = factory
}

// CreateStore creates a store instance if it belongs to a registered type.
func CreateStore(opts CreateStorOptions) (Store, error) {
	if opts.Name == "" || opts.Type == "" {
		return nil, fmt.Errorf("name or type is not provided in the store config")
	}
	storeFactory, ok := registeredStores[opts.Type]
	if ok {
		return storeFactory(opts)
	}
	return nil, fmt.Errorf("store factory of type %s is not registered", opts.Type)
}
