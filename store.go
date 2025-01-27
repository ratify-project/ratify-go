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

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

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
	// underlying pagination.
	ListReferrers(ctx context.Context, ref string, artifactTypes []string, fn func(referrers []ocispec.Descriptor) error) error

	// FetchBlob returns the blob by the given reference.
	// WARNING: This API is intended to use for small objects like signatures,
	// SBoMs.
	FetchBlob(ctx context.Context, repo string, desc ocispec.Descriptor) ([]byte, error)

	// FetchManifest returns the referenced image manifest as given by the
	// descriptor.
	FetchManifest(ctx context.Context, repo string, desc ocispec.Descriptor) ([]byte, error)
}
