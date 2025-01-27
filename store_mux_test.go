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
	"reflect"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type testStore struct {
	t             *testing.T
	name          string
	resolve       func(*testing.T, string) (ocispec.Descriptor, error)
	listReferrers func(*testing.T, string, []string, func([]ocispec.Descriptor) error) error
	fetchBlob     func(*testing.T, string, ocispec.Descriptor) ([]byte, error)
	fetchManifest func(*testing.T, string, ocispec.Descriptor) ([]byte, error)
}

func (s *testStore) Name() string {
	return s.name
}

func (s *testStore) Resolve(ctx context.Context, ref string) (ocispec.Descriptor, error) {
	if s.resolve == nil {
		s.t.Fatal("unexpected call to Store.Resolve")
	}
	return s.resolve(s.t, ref)
}

func (s *testStore) ListReferrers(ctx context.Context, ref string, artifactTypes []string, fn func([]ocispec.Descriptor) error) error {
	if s.listReferrers == nil {
		s.t.Fatal("unexpected call to Store.ListReferrers")
	}
	return s.listReferrers(s.t, ref, artifactTypes, fn)
}

func (s *testStore) FetchBlob(ctx context.Context, repo string, desc ocispec.Descriptor) ([]byte, error) {
	if s.fetchBlob == nil {
		s.t.Fatal("unexpected call to Store.FetchBlob")
	}
	return s.fetchBlob(s.t, repo, desc)
}

func (s *testStore) FetchManifest(ctx context.Context, repo string, desc ocispec.Descriptor) ([]byte, error) {
	if s.fetchManifest == nil {
		s.t.Fatal("unexpected call to Store.FetchManifest")
	}
	return s.fetchManifest(s.t, repo, desc)
}

func TestStoreMux(t *testing.T) {
	var store any = &StoreMux{}
	if _, ok := store.(Store); !ok {
		t.Error("*StoreMux does not implement Store")
	}
}

func TestStoreMux_Name(t *testing.T) {
	want := "test"
	s := NewStoreMux(want)
	if got := s.Name(); got != want {
		t.Errorf("StoreMux.Name() = %v, want %v", got, want)
	}
}

func TestStoreMux_Resolve(t *testing.T) {
	ctx := context.Background()
	const pattern = "registry.test"

	tests := []struct {
		name    string
		ref     string
		resolve func(*testing.T, string) (ocispec.Descriptor, error)
		want    ocispec.Descriptor
		wantErr bool
	}{
		{
			name: "match store",
			ref:  "registry.test/foo:v1",
			resolve: func(t *testing.T, ref string) (ocispec.Descriptor, error) {
				return ocispec.DescriptorEmptyJSON, nil
			},
			want: ocispec.DescriptorEmptyJSON,
		},
		{
			name:    "no matching store found",
			ref:     "null.test/foo:v1",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStoreMux("hello")
			if err := store.Register(pattern, &testStore{
				t:       t,
				resolve: tt.resolve,
			}); err != nil {
				t.Fatalf("StoreMux.Register() error = %s, want nil", err)
			}
			got, err := store.Resolve(ctx, tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("StoreMux.Resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StoreMux.Resolve() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStoreMux_ListReferrers(t *testing.T) {
	ctx := context.Background()
	const pattern = "registry.test"

	tests := []struct {
		name          string
		ref           string
		listReferrers func(*testing.T, string, []string, func([]ocispec.Descriptor) error) error
		want          []ocispec.Descriptor
		wantErr       bool
	}{
		{
			name: "match store",
			ref:  "registry.test/foo:v1",
			listReferrers: func(t *testing.T, _ string, _ []string, fn func([]ocispec.Descriptor) error) error {
				return fn([]ocispec.Descriptor{ocispec.DescriptorEmptyJSON})
			},
			want: []ocispec.Descriptor{ocispec.DescriptorEmptyJSON},
		},
		{
			name:    "no matching store found",
			ref:     "null.test/foo:v1",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStoreMux("hello")
			if err := store.Register(pattern, &testStore{
				t:             t,
				listReferrers: tt.listReferrers,
			}); err != nil {
				t.Fatalf("StoreMux.Register() error = %s, want nil", err)
			}
			var got []ocispec.Descriptor
			fn := func(referrers []ocispec.Descriptor) error {
				got = append(got, referrers...)
				return nil
			}
			if err := store.ListReferrers(ctx, tt.ref, nil, fn); (err != nil) != tt.wantErr {
				t.Errorf("StoreMux.ListReferrers() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StoreMux.ListReferrers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStoreMux_FetchBlob(t *testing.T) {
	ctx := context.Background()
	const pattern = "registry.test"

	tests := []struct {
		name      string
		repo      string
		fetchBlob func(*testing.T, string, ocispec.Descriptor) ([]byte, error)
		want      []byte
		wantErr   bool
	}{
		{
			name: "match store",
			repo: "registry.test/foo",
			fetchBlob: func(t *testing.T, _ string, _ ocispec.Descriptor) ([]byte, error) {
				return []byte("foo"), nil
			},
			want: []byte("foo"),
		},
		{
			name:    "no matching store found",
			repo:    "null.test/foo",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStoreMux("hello")
			if err := store.Register(pattern, &testStore{
				t:         t,
				fetchBlob: tt.fetchBlob,
			}); err != nil {
				t.Fatalf("StoreMux.Register() error = %s, want nil", err)
			}
			got, err := store.FetchBlob(ctx, tt.repo, ocispec.Descriptor{})
			if (err != nil) != tt.wantErr {
				t.Errorf("StoreMux.FetchBlob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StoreMux.FetchBlob() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStoreMux_FetchManifest(t *testing.T) {
	ctx := context.Background()
	const pattern = "registry.test"

	tests := []struct {
		name          string
		repo          string
		fetchManifest func(*testing.T, string, ocispec.Descriptor) ([]byte, error)
		want          []byte
		wantErr       bool
	}{
		{
			name: "match store",
			repo: "registry.test/foo",
			fetchManifest: func(t *testing.T, _ string, _ ocispec.Descriptor) ([]byte, error) {
				return []byte("foo"), nil
			},
			want: []byte("foo"),
		},
		{
			name:    "no matching store found",
			repo:    "null.test/foo",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStoreMux("hello")
			if err := store.Register(pattern, &testStore{
				t:             t,
				fetchManifest: tt.fetchManifest,
			}); err != nil {
				t.Fatalf("StoreMux.Register() error = %s, want nil", err)
			}
			got, err := store.FetchManifest(ctx, tt.repo, ocispec.Descriptor{})
			if (err != nil) != tt.wantErr {
				t.Errorf("StoreMux.FetchManifest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StoreMux.FetchManifest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStoreMux_Register(t *testing.T) {
	type subTest struct {
		name    string
		ref     string
		wantErr bool
	}
	tests := []struct {
		name    string
		pattern string
		tests   []subTest
		wantErr bool
	}{
		{
			name:    "empty pattern",
			pattern: "",
			wantErr: true,
		},
		{
			name:    "exact repository name",
			pattern: "registry.test/foo",
			tests: []subTest{
				{
					name: "match",
					ref:  "registry.test/foo:v1",
				},
				{
					name:    "no match",
					ref:     "registry.test/bar:v1",
					wantErr: true,
				},
			},
		},
		{
			name:    "exact registry name",
			pattern: "registry.test",
			tests: []subTest{
				{
					name: "match",
					ref:  "registry.test/foo:v1",
				},
				{
					name:    "no match",
					ref:     "registry.example/foo:v1",
					wantErr: true,
				},
			},
		},
		{
			name:    "wildcard registry name",
			pattern: "*.test",
			tests: []subTest{
				{
					name: "match",
					ref:  "registry.test/foo:v1",
				},
				{
					name:    "no match",
					ref:     "registry.example/foo:v1",
					wantErr: true,
				},
				{
					name:    "wildcard match in the middle not allowed",
					ref:     "another.registry.test/foo:v1",
					wantErr: true,
				},
			},
		},
		{
			name:    "top level wildcard registry name not allowed",
			pattern: "*",
			wantErr: true,
		},
		{
			name:    "multiple wildcard registry name not allowed",
			pattern: "*.*.example",
			wantErr: true,
		},
		{
			name:    "invalid wildcard registry name not allowed",
			pattern: "example.*.test",
			wantErr: true,
		},
		{
			name:    "wildcard registry name with exact repository name not allowed",
			pattern: "*.test/foo",
			wantErr: true,
		},
		{
			name:    "invalid registry name",
			pattern: "example:abc",
			wantErr: true,
		},
		{
			name:    "invalid repository name",
			pattern: "registry.test/FOO",
			wantErr: true,
		},
		{
			name:    "register full reference",
			pattern: "registry.test/foo:v1",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewStoreMux("hello")
			want := &testStore{t: t}
			if err := store.Register(tt.pattern, want); (err != nil) != tt.wantErr {
				t.Errorf("StoreMux.Register() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for _, ttt := range tt.tests {
				t.Run(ttt.name, func(t *testing.T) {
					got, err := store.storeFromReference(ttt.ref)
					if (err != nil) != ttt.wantErr {
						t.Errorf("StoreMux.storeFromReference() error = %v, wantErr %v", err, ttt.wantErr)
						return
					}
					if !ttt.wantErr && got != want {
						t.Errorf("StoreMux.storeFromReference() = %v, want %v", got, want)
					}
				})
			}
		})
	}

	t.Run("nil store", func(t *testing.T) {
		store := NewStoreMux("hello")
		if err := store.Register("registry.test", nil); err == nil {
			t.Error("StoreMux.Register() error = nil, wantErr")
		}
	})
}

func TestStoreMux_RegisterFallback(t *testing.T) {
	t.Run("success fallback", func(t *testing.T) {
		store := NewStoreMux("hello")
		want := &testStore{t: t}
		if err := store.RegisterFallback(want); err != nil {
			t.Errorf("StoreMux.RegisterFallback() error = %v, wantErr nil", err)
			return
		}
		got, err := store.storeFromReference("registry.test/foo:v1")
		if err != nil {
			t.Errorf("StoreMux.storeFromReference() error = %v, wantErr nil", err)
			return
		}
		if got != want {
			t.Errorf("StoreMux.storeFromReference() = %v, want %v", got, want)
		}
	})

	t.Run("nil store", func(t *testing.T) {
		store := NewStoreMux("hello")
		if err := store.RegisterFallback(nil); err == nil {
			t.Error("StoreMux.RegisterFallback() error = nil, wantErr")
		}
	})
}
