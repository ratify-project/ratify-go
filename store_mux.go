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
	"errors"
	"fmt"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry"
)

// StoreMux is a store multiplexer.
// It matches the registry name of each incoming request against a list of
// registered patterns and calls the store for the pattern that most closely
// matches the registry name.
//
// # Patterns
//
// Patterns can match the registry name and the repository name.
// If patterns are registered only with the registry name, wildcard DNS records
// (see RFC 1034, Section 4.3.3, RFC 4592, and RFC 6125, Section 6.4.3) are
// accepted.
// Some examples:
//
//   - "registry.example" matches any request to the host "registry.example"
//   - "registry.example/foo" matches any request to the repository "foo" on the
//     host "registry.example"
//   - "*.example" matches any request to any host in the "example" domain
//   - "*.example:5000" matches any request to the host "*.example" on port 5000
//
// Patterns with repository names does not support wildcard DNS records.
// For example, "*.example/foo" is not a valid pattern.
// Top level domain wildcard is also not supported. That is, "*" is not a valid
// pattern.
//
// # Precedence
//
// If two or more patterns match a request, then the most specific pattern takes
// precedence. For example, if both "registry.example/foo" and "registry.example"
// are registered, then the former takes precedence.
type StoreMux struct {
	name       string
	wildcard   map[string]Store
	registry   map[string]Store
	repository map[string]Store
	fallback   Store
}

// NewStoreMux creates a new [StoreMux].
func NewStoreMux(name string) *StoreMux {
	return &StoreMux{
		name: name,
	}
}

// Name is the name of the store.
func (s *StoreMux) Name() string {
	return s.name
}

// Resolve resolves to a descriptor for the given artifact reference.
func (s *StoreMux) Resolve(ctx context.Context, ref string) (ocispec.Descriptor, error) {
	store, err := s.storeFromReference(ref)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	return store.Resolve(ctx, ref)
}

// ListReferrers returns the immediate set of supply chain artifacts for the
// given subject reference.
// Note: This API supports pagination. fn should be set to handle the
// underlying pagination.
func (s *StoreMux) ListReferrers(ctx context.Context, ref string, artifactTypes []string, fn func(referrers []ocispec.Descriptor) error) error {
	store, err := s.storeFromReference(ref)
	if err != nil {
		return err
	}
	return store.ListReferrers(ctx, ref, artifactTypes, fn)
}

// FetchBlob returns the blob by the given reference.
// WARNING: This API is intended to use for small objects like signatures,
// SBoMs.
func (s *StoreMux) FetchBlob(ctx context.Context, repo string, desc ocispec.Descriptor) ([]byte, error) {
	store, err := s.storeFromRepository(repo)
	if err != nil {
		return nil, err
	}
	return store.FetchBlob(ctx, repo, desc)
}

// FetchManifest returns the referenced manifest as given by the descriptor.
func (s *StoreMux) FetchManifest(ctx context.Context, repo string, desc ocispec.Descriptor) ([]byte, error) {
	store, err := s.storeFromRepository(repo)
	if err != nil {
		return nil, err
	}
	return store.FetchManifest(ctx, repo, desc)
}

// Register registers a store for the given pattern.
func (s *StoreMux) Register(pattern string, store Store) error {
	if pattern == "" {
		return errors.New("pattern is required")
	}
	if store == nil {
		return errors.New("store is required")
	}

	// check for FQDN pattern
	if strings.Contains(pattern, "/") {
		return s.registerRepository(pattern, store)
	}

	// check for registry pattern
	return s.registerRegistry(pattern, store)
}

// registerRegistry registers a store for the given registry pattern.
func (s *StoreMux) registerRegistry(pattern string, store Store) error {
	ref := registry.Reference{
		Registry: pattern,
	}
	if err := ref.ValidateRegistry(); err != nil {
		return fmt.Errorf("invalid pattern %q: %v", pattern, err)
	}

	switch strings.Count(pattern, "*") {
	case 0:
		if s.registry == nil {
			s.registry = map[string]Store{
				pattern: store,
			}
		} else {
			s.registry[pattern] = store
		}
	case 1:
		if !strings.HasPrefix(pattern, "*.") {
			return fmt.Errorf("invalid pattern %q: wildcard DNS records must start with '*.'", pattern)
		}
		pattern = pattern[2:]
		if s.wildcard == nil {
			s.wildcard = map[string]Store{
				pattern: store,
			}
		} else {
			s.wildcard[pattern] = store
		}
	default:
		return fmt.Errorf("invalid pattern %q: too many wildcards", pattern)
	}

	return nil
}

// registerRepository registers a store for the given repository pattern.
func (s *StoreMux) registerRepository(pattern string, store Store) error {
	if strings.Contains(pattern, "*") {
		return fmt.Errorf("invalid pattern %q: wildcard DNS records not supported", pattern)
	}
	ref, err := registry.ParseReference(pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern %q: %v", pattern, err)
	}
	if ref.Reference != "" {
		return fmt.Errorf("invalid pattern %q: tag or digest not supported", pattern)
	}

	if s.repository == nil {
		s.repository = map[string]Store{
			pattern: store,
		}
	} else {
		s.repository[pattern] = store
	}

	return nil
}

// RegisterFallback registers a fallback store, which is used if no other store
// matches.
func (s *StoreMux) RegisterFallback(store Store) error {
	if store == nil {
		return errors.New("store is required")
	}
	s.fallback = store
	return nil
}

// storeFromReference returns the store for the given reference.
func (s *StoreMux) storeFromReference(ref string) (Store, error) {
	// optimize if no repository patterns are registered.
	if len(s.repository) == 0 {
		registry, _, _ := strings.Cut(ref, "/")
		return s.storeFromRegistry(registry)
	}

	reference, err := registry.ParseReference(ref)
	if err != nil {
		return nil, err
	}
	repository := reference.Registry + "/" + reference.Repository
	return s.storeFromRepository(repository)
}

// storeFromRepository returns the store for the given repository.
func (s *StoreMux) storeFromRepository(repo string) (Store, error) {
	// check for exact match.
	if store, ok := s.repository[repo]; ok {
		return store, nil
	}

	// check for registry.
	registry, _, _ := strings.Cut(repo, "/")
	return s.storeFromRegistry(registry)
}

// storeFromRegistry returns the store for the given registry.
func (s *StoreMux) storeFromRegistry(registry string) (Store, error) {
	// check for exact match.
	if store, ok := s.registry[registry]; ok {
		return store, nil
	}

	// check for wildcard match.
	if _, zone, ok := strings.Cut(registry, "."); ok {
		if store, ok := s.wildcard[zone]; ok {
			return store, nil
		}
	}
	// check for fallback.
	if s.fallback != nil {
		return s.fallback, nil
	}

	return nil, errors.New("no matching store found")
}
