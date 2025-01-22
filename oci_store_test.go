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
	"os"
	"reflect"
	"sync/atomic"
	"testing"
	"testing/fstest"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
)

func TestNewOCIStoreFromFS(t *testing.T) {
	ctx := context.Background()
	const name = "test"

	t.Run("valid folder", func(t *testing.T) {
		fsys := fstest.MapFS{
			"oci-layout": &fstest.MapFile{
				Data: []byte(`{"imageLayoutVersion":"1.0.0"}`),
			},
			"index.json": &fstest.MapFile{
				Data: []byte(`{"schemaVersion":2,"manifests":[]}`),
			},
		}
		_, err := NewOCIStoreFromFS(ctx, name, fsys)
		if err != nil {
			t.Errorf("NewOCIStoreFromFS() error = %v, wantErr false", err)
			return
		}
	})

	t.Run("non-OCI folder", func(t *testing.T) {
		var fsys fstest.MapFS
		_, err := NewOCIStoreFromFS(ctx, name, fsys)
		if err == nil {
			t.Errorf("NewOCIStoreFromFS() error = nil, wantErr true")
			return
		}
	})
}

func TestOCIStore_Name(t *testing.T) {
	ctx := context.Background()
	fsys := os.DirFS("testdata/oci_store/hello")
	want := "test"
	s, err := NewOCIStoreFromFS(ctx, want, fsys)
	if err != nil {
		t.Fatalf("NewOCIStoreFromFS() error = %v, want nil", err)
	}
	if got := s.Name(); got != want {
		t.Errorf("OCIStore.Name() = %v, want %v", got, want)
	}
}

func TestOCIStore_Resolve(t *testing.T) {
	ctx := context.Background()
	fsys := os.DirFS("testdata/oci_store/hello")
	store, err := NewOCIStoreFromFS(ctx, "hello", fsys)
	if err != nil {
		t.Fatalf("NewOCIStoreFromFS() error = %v, want nil", err)
	}
	desc := ocispec.Descriptor{
		MediaType: "application/vnd.oci.image.manifest.v1+json",
		Digest:    "sha256:2b858809d6fd3d63a2e64e8418a0d5883aec3e24e4fe6346370f09e043763b83",
		Size:      588,
	}

	tests := []struct {
		name    string
		ref     string
		want    ocispec.Descriptor
		wantErr bool
	}{
		{
			name: "valid ref",
			ref:  "v1",
			want: desc,
		},
		{
			name: "digest ref",
			ref:  "sha256:2b858809d6fd3d63a2e64e8418a0d5883aec3e24e4fe6346370f09e043763b83",
			want: desc,
		},
		{
			name: "full ref",
			ref:  "localhost:5000/hello:v1",
			want: desc,
		},
		{
			name: "full digest ref",
			ref:  "localhost:5000/hello@sha256:2b858809d6fd3d63a2e64e8418a0d5883aec3e24e4fe6346370f09e043763b83",
			want: desc,
		},
		{
			name:    "non-existing ref",
			ref:     "v2",
			wantErr: true,
		},
		{
			name:    "non-existing digest ref",
			ref:     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.Resolve(ctx, tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("OCIStore.Resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !content.Equal(got, tt.want) {
				t.Errorf("OCIStore.Resolve() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOCIStore_ListReferrers(t *testing.T) {
	ctx := context.Background()
	fsys := os.DirFS("testdata/oci_store/hello")
	store, err := NewOCIStoreFromFS(ctx, "hello", fsys)
	if err != nil {
		t.Fatalf("NewOCIStoreFromFS() error = %v, want nil", err)
	}
	fooDesc := ocispec.Descriptor{
		MediaType:    "application/vnd.oci.image.manifest.v1+json",
		Digest:       "sha256:429c5e7304ab01d7d5d3772bc1b4443844e0b2b8f6cd362eac2a7f83598b4927",
		Size:         896,
		ArtifactType: "application/foo",
		Annotations: map[string]string{
			"org.opencontainers.image.created": "2025-01-22T09:54:49Z",
		},
	}
	barDesc := ocispec.Descriptor{
		MediaType:    "application/vnd.oci.image.manifest.v1+json",
		Digest:       "sha256:0869171bd25813427afe0619ab8f2dc35a95527f24c5806580b37fe88cbbd105",
		Size:         896,
		ArtifactType: "application/bar",
		Annotations: map[string]string{
			"org.opencontainers.image.created": "2025-01-22T09:55:25Z",
		},
	}

	tests := []struct {
		name          string
		ref           string
		artifactTypes []string
		want          []ocispec.Descriptor
		wantErr       bool
	}{
		{
			name: "list all referrers",
			ref:  "v1",
			want: []ocispec.Descriptor{fooDesc, barDesc},
		},
		{
			name:          "list referrers of certain type",
			ref:           "v1",
			artifactTypes: []string{fooDesc.ArtifactType},
			want:          []ocispec.Descriptor{fooDesc},
		},
		{
			name:          "list referrers of non-existing type",
			ref:           "v1",
			artifactTypes: []string{"application/test"},
			want:          nil,
		},
		{
			name:          "list referrers of partial non-existing type",
			ref:           "v1",
			artifactTypes: []string{fooDesc.ArtifactType, "application/test"},
			want:          []ocispec.Descriptor{fooDesc},
		},
		{
			name: "non-existing ref",
			ref:  "v2",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []ocispec.Descriptor
			var fnCount int32
			fn := func(referrers []ocispec.Descriptor) error {
				atomic.AddInt32(&fnCount, 1)
				got = referrers
				return nil
			}
			if err := store.ListReferrers(ctx, tt.ref, tt.artifactTypes, fn); (err != nil) != tt.wantErr {
				t.Errorf("OCIStore.ListReferrers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if fnCount > 1 {
				t.Errorf("OCIStore.ListReferrers() count(fn) = %v, want 1", fnCount)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OCIStore.ListReferrers() = %v, want %v", got, tt.want)
			}
		})
	}
}
