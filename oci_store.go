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
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"slices"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry"
)

// OCIStore is a read-only store implementation based on OCI image layout.
type OCIStore struct {
	name  string
	store *oci.ReadOnlyStore
}

// NewOCIStoreFromFS creates a new [OCIStore] from the given filesystem in OCI
// image layout.
func NewOCIStoreFromFS(ctx context.Context, name string, fsys fs.FS) (*OCIStore, error) {
	store, err := oci.NewFromFS(ctx, fsys)
	if err != nil {
		return nil, err
	}
	return &OCIStore{
		name:  name,
		store: store,
	}, nil
}

// NewOCIStoreFromTar creates a new [OCIStore] from the given tarball in OCI
// image layout.
func NewOCIStoreFromTar(ctx context.Context, name, path string) (*OCIStore, error) {
	store, err := oci.NewFromTar(ctx, path)
	if err != nil {
		return nil, err
	}
	return &OCIStore{
		name:  name,
		store: store,
	}, nil
}

// Name is the name of the store.
func (s *OCIStore) Name() string {
	return s.name
}

// Resolve resolves to a descriptor for the given artifact reference.
func (s *OCIStore) Resolve(ctx context.Context, ref string) (ocispec.Descriptor, error) {
	desc, err := s.store.Resolve(ctx, ref)
	if err != nil {
		if !errors.Is(err, errdef.ErrNotFound) {
			return ocispec.Descriptor{}, err
		}

		// if there is no exact match on the full reference, try to resolve the
		// tag / digest only.
		reference, refErr := registry.ParseReference(ref)
		if refErr == nil && reference.Reference != "" {
			return s.store.Resolve(ctx, reference.Reference)
		}
		return ocispec.Descriptor{}, err
	}
	return desc, nil
}

// ListReferrers returns the immediate set of supply chain artifacts for the
// given subject reference.
func (s *OCIStore) ListReferrers(ctx context.Context, ref string, artifactTypes []string, fn func(referrers []ocispec.Descriptor) error) error {
	desc, err := s.Resolve(ctx, ref)
	if err != nil {
		// return empty referrer list if the subject is not found.
		// reference: https://github.com/opencontainers/distribution-spec/blob/v1.1.0/spec.md#listing-referrers
		if errors.Is(err, errdef.ErrNotFound) {
			return nil
		}
		return err
	}

	referrers, err := registry.Referrers(ctx, s.store, desc, "")
	if err != nil {
		return err
	}

	// filter referrers by artifact types if specified.
	if len(artifactTypes) > 0 {
		referrers = slices.DeleteFunc(referrers, func(desc ocispec.Descriptor) bool {
			return !slices.Contains(artifactTypes, desc.ArtifactType)
		})
	}

	// report the referrers.
	if len(referrers) == 0 {
		return nil
	}
	return fn(referrers)
}

// FetchBlobContent returns the blob by the given reference.
func (s *OCIStore) FetchBlobContent(ctx context.Context, _ string, desc ocispec.Descriptor) ([]byte, error) {
	return content.FetchAll(ctx, s.store, desc)
}

// FetchImageManifest returns the referenced image manifest as given by the
// descriptor.
func (s *OCIStore) FetchImageManifest(ctx context.Context, _ string, desc ocispec.Descriptor) (*ocispec.Manifest, error) {
	manfiestBytes, err := content.FetchAll(ctx, s.store, desc)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	var manifest ocispec.Manifest
	if err := json.Unmarshal(manfiestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}
	return &manifest, nil
}
