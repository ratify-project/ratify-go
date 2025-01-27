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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

func TestRegistryStore(t *testing.T) {
	var store any = &RegistryStore{}
	if _, ok := store.(Store); !ok {
		t.Error("*RegistryStore does not implement Store")
	}
}

func TestNewRegistryStore(t *testing.T) {
	const name = "test"

	t.Run("create store with default settings", func(t *testing.T) {
		s, err := NewRegistryStore(name, RegistryStoreOptions{})
		if err != nil {
			t.Fatalf("NewRegistryStore() error = %v, want nil", err)
		}
		if s.name != name {
			t.Errorf("RegistryStore.name = %v, want %v", s.name, name)
		}
		if s.client.Client != nil {
			t.Errorf("RegistryStore.client.Client = %v, want nil", s.client.Client)
		}
		if s.plainHTTP != false {
			t.Errorf("RegistryStore.plainHTTP = %v, want false", s.plainHTTP)
		}
		if userAgent := s.client.Header.Get("User-Agent"); userAgent != defaultUserAgent {
			t.Errorf(`RegistryStore.client.Header["User-Agent"] = %v, want %v`, userAgent, defaultUserAgent)
		}
		if credential := s.client.Credential; credential != nil {
			t.Errorf("RegistryStore.client.Credential = %v, want nil", credential)
		}
		if s.maxBlobBytes != defaultMaxBlobBytes {
			t.Errorf("RegistryStore.maxBlobBytes = %v, want %v", s.maxBlobBytes, defaultMaxBlobBytes)
		}
		if s.maxManifestBytes != defaultMaxManifestBytes {
			t.Errorf("RegistryStore.maxManifestBytes = %v, want %v", s.maxManifestBytes, defaultMaxManifestBytes)
		}
	})

	t.Run("create store with custom settings", func(t *testing.T) {
		opts := RegistryStoreOptions{
			HTTPClient:         &http.Client{},
			PlainHTTP:          true,
			UserAgent:          "test-agent",
			CredentialProvider: credentials.NewMemoryStore(),
			MaxBlobBytes:       2048,
			MaxManifestBytes:   1024,
		}
		s, err := NewRegistryStore(name, opts)
		if err != nil {
			t.Fatalf("NewRegistryStore() error = %v, want nil", err)
		}
		if s.name != name {
			t.Errorf("RegistryStore.name = %v, want %v", s.name, name)
		}
		if s.client.Client != opts.HTTPClient {
			t.Errorf("RegistryStore.client.Client = %v, want %v", s.client.Client, opts.HTTPClient)
		}
		if s.plainHTTP != opts.PlainHTTP {
			t.Errorf("RegistryStore.plainHTTP = %v, want %v", s.plainHTTP, opts.PlainHTTP)
		}
		if userAgent := s.client.Header.Get("User-Agent"); userAgent != opts.UserAgent {
			t.Errorf(`RegistryStore.client.Header["User-Agent"] = %v, want %v`, userAgent, opts.UserAgent)
		}
		if credential := s.client.Credential; credential == nil {
			t.Errorf("RegistryStore.client.Credential = nil, want non-nil")
		}
		if s.maxBlobBytes != opts.MaxBlobBytes {
			t.Errorf("RegistryStore.maxBlobBytes = %v, want %v", s.maxBlobBytes, opts.MaxBlobBytes)
		}
		if s.maxManifestBytes != opts.MaxManifestBytes {
			t.Errorf("RegistryStore.maxManifestBytes = %v, want %v", s.maxManifestBytes, opts.MaxManifestBytes)
		}
	})

	t.Run("missing store name", func(t *testing.T) {
		_, err := NewRegistryStore("", RegistryStoreOptions{})
		if err == nil {
			t.Errorf("NewRegistryStore() error = nil, wantErr true")
		}
	})
}

func TestRegistryStore_Name(t *testing.T) {
	want := "test"

	s, err := NewRegistryStore(want, RegistryStoreOptions{})
	if err != nil {
		t.Fatalf("NewRegistryStore() error = %v, want nil", err)
	}
	if got := s.Name(); got != want {
		t.Errorf("RegistryStore.Name() = %v, want %v", got, want)
	}
}

func TestRegistryStore_Resolve(t *testing.T) {
	blob := []byte("hello world")
	blobDesc := content.NewDescriptorFromBytes("test", blob)
	index := []byte(`{"manifests":[]}`)
	indexDesc := content.NewDescriptorFromBytes(ocispec.MediaTypeImageIndex, index)
	ref := "v1"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("unexpected access: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/v2/test/manifests/" + blobDesc.Digest.String(),
			"/v2/test/manifests/v2":
			w.WriteHeader(http.StatusNotFound)
		case "/v2/test/manifests/" + indexDesc.Digest.String(),
			"/v2/test/manifests/" + ref:
			if accept := r.Header.Get("Accept"); !strings.Contains(accept, indexDesc.MediaType) {
				t.Errorf("manifest not convertible: %s", accept)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", indexDesc.MediaType)
			w.Header().Set("Docker-Content-Digest", indexDesc.Digest.String())
			w.Header().Set("Content-Length", strconv.Itoa(int(indexDesc.Size)))
		default:
			t.Errorf("unexpected access: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()
	uri, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("invalid test http server: %v", err)
	}
	repoName := uri.Host + "/test"
	store, err := NewRegistryStore("hello", RegistryStoreOptions{
		PlainHTTP: true,
	})
	if err != nil {
		t.Fatalf("NewRegistryStore() error = %v, want nil", err)
	}
	ctx := context.Background()

	tests := []struct {
		name    string
		ref     string
		want    ocispec.Descriptor
		wantErr bool
	}{
		{
			name: "tag ref",
			ref:  repoName + ":v1",
			want: indexDesc,
		},
		{
			name: "digest ref",
			ref:  repoName + "@" + indexDesc.Digest.String(),
			want: indexDesc,
		},
		{
			name:    "non-existing ref",
			ref:     repoName + ":v2",
			wantErr: true,
		},
		{
			name:    "non-existing digest ref",
			ref:     repoName + "@" + blobDesc.Digest.String(),
			wantErr: true,
		},
		{
			name:    "invalid ref",
			ref:     "invalid",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.Resolve(ctx, tt.ref)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegistryStore.Resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RegistryStore.Resolve() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegistryStore_ListReferrers(t *testing.T) {
	manifest := []byte(`{"layers":[]}`)
	manifestDesc := content.NewDescriptorFromBytes(ocispec.MediaTypeImageManifest, manifest)
	referrerSet := [][]ocispec.Descriptor{
		{
			{
				MediaType:    ocispec.MediaTypeImageManifest,
				Size:         1,
				Digest:       digest.FromString("1"),
				ArtifactType: "application/foo",
			},
			{
				MediaType:    ocispec.MediaTypeImageManifest,
				Size:         2,
				Digest:       digest.FromString("2"),
				ArtifactType: "application/bar",
			},
		},
		{
			{
				MediaType:    ocispec.MediaTypeImageManifest,
				Size:         3,
				Digest:       digest.FromString("3"),
				ArtifactType: "application/foo",
			},
			{
				MediaType:    ocispec.MediaTypeImageManifest,
				Size:         4,
				Digest:       digest.FromString("4"),
				ArtifactType: "application/bar",
			},
		},
		{
			{
				MediaType:    ocispec.MediaTypeImageManifest,
				Size:         5,
				Digest:       digest.FromString("5"),
				ArtifactType: "application/foo",
			},
		},
	}
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			switch r.URL.Path {
			case "/v2/test/manifests/v1":
				if accept := r.Header.Get("Accept"); !strings.Contains(accept, manifestDesc.MediaType) {
					t.Errorf("manifest not convertible: %s", accept)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", manifestDesc.MediaType)
				w.Header().Set("Docker-Content-Digest", manifestDesc.Digest.String())
				w.Header().Set("Content-Length", strconv.Itoa(int(manifestDesc.Size)))
				return
			case "/v2/test/manifests/v2":
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}

		path := "/v2/test/referrers/" + manifestDesc.Digest.String()
		if r.Method != http.MethodGet || r.URL.Path != path {
			referrersTag := strings.Replace(manifestDesc.Digest.String(), ":", "-", 1)
			if r.URL.Path != "/v2/test/manifests/"+referrersTag {
				t.Errorf("unexpected access: %s %q", r.Method, r.URL)
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}
		q := r.URL.Query()
		var referrers []ocispec.Descriptor
		switch artifactType := q.Get("artifactType"); artifactType {
		case "":
			switch q.Get("test") {
			case "foo":
				referrers = referrerSet[1]
				w.Header().Set("Link", fmt.Sprintf(`<%s%s?n=2&test=bar>; rel="next"`, ts.URL, path))
			case "bar":
				referrers = referrerSet[2]
			default:
				referrers = referrerSet[0]
				w.Header().Set("Link", fmt.Sprintf(`<%s?n=2&test=foo>; rel="next"`, path))
			}
		case "application/foo":
			referrers = []ocispec.Descriptor{
				referrerSet[0][0],
				referrerSet[1][0],
				referrerSet[2][0],
			}
		default:
			referrers = []ocispec.Descriptor{}
		}
		result := ocispec.Index{
			Versioned: specs.Versioned{
				SchemaVersion: 2, // historical value. does not pertain to OCI or docker version
			},
			MediaType: ocispec.MediaTypeImageIndex,
			Manifests: referrers,
		}
		w.Header().Set("Content-Type", ocispec.MediaTypeImageIndex)
		if err := json.NewEncoder(w).Encode(result); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer ts.Close()
	uri, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("invalid test http server: %v", err)
	}
	repoName := uri.Host + "/test"
	store, err := NewRegistryStore("hello", RegistryStoreOptions{
		PlainHTTP: true,
	})
	if err != nil {
		t.Fatalf("NewRegistryStore() error = %v, want nil", err)
	}
	ctx := context.Background()

	tests := []struct {
		name          string
		ref           string
		artifactTypes []string
		want          []ocispec.Descriptor
		wantFnCount   int32
		wantErr       bool
	}{
		{
			name:        "list all referrers",
			ref:         repoName + ":v1",
			want:        slices.Concat(referrerSet...),
			wantFnCount: 3,
		},
		{
			name:        "list all referrers of a digest reference",
			ref:         repoName + "@" + manifestDesc.Digest.String(),
			want:        slices.Concat(referrerSet...),
			wantFnCount: 3,
		},
		{
			name:          "list referrers of certain type",
			ref:           repoName + ":v1",
			artifactTypes: []string{"application/foo"},
			want: []ocispec.Descriptor{
				referrerSet[0][0],
				referrerSet[1][0],
				referrerSet[2][0],
			},
			wantFnCount: 1,
		},
		{
			name:          "list referrers of non-existing type",
			ref:           repoName + ":v1",
			artifactTypes: []string{"application/test"},
			want:          nil,
		},
		{
			name:          "list referrers of partial non-existing type",
			ref:           repoName + ":v1",
			artifactTypes: []string{"application/foo", "application/test"},
			want: []ocispec.Descriptor{
				referrerSet[0][0],
				referrerSet[1][0],
				referrerSet[2][0],
			},
			wantFnCount: 3,
		},
		{
			name: "non-existing ref",
			ref:  repoName + ":v2",
			want: nil,
		},
		{
			name:    "invalid ref",
			ref:     "invalid",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []ocispec.Descriptor
			var fnCount int32
			fn := func(referrers []ocispec.Descriptor) error {
				atomic.AddInt32(&fnCount, 1)
				got = append(got, referrers...)
				return nil
			}
			if err := store.ListReferrers(ctx, tt.ref, tt.artifactTypes, fn); (err != nil) != tt.wantErr {
				t.Errorf("RegistryStore.ListReferrers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if fnCount != tt.wantFnCount {
				t.Errorf("RegistryStore.ListReferrers() count(fn) = %v, want %v", fnCount, tt.wantFnCount)
			}
			slices.SortFunc(got, func(a, b ocispec.Descriptor) int {
				return int(a.Size - b.Size)
			})
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OCIStore.ListReferrers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegistryStore_FetchBlob(t *testing.T) {
	blob := []byte("hello world")
	blobDesc := content.NewDescriptorFromBytes("test", blob)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected access: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/v2/test/blobs/" + blobDesc.Digest.String():
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Docker-Content-Digest", blobDesc.Digest.String())
			if _, err := w.Write(blob); err != nil {
				t.Errorf("failed to write %q: %v", r.URL, err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()
	uri, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("invalid test http server: %v", err)
	}
	repoName := uri.Host + "/test"
	store, err := NewRegistryStore("hello", RegistryStoreOptions{
		PlainHTTP: true,
	})
	if err != nil {
		t.Fatalf("NewRegistryStore() error = %v, want nil", err)
	}
	ctx := context.Background()

	tests := []struct {
		name    string
		repo    string
		desc    ocispec.Descriptor
		want    []byte
		wantErr bool
	}{
		{
			name: "fetch blob",
			repo: repoName,
			desc: blobDesc,
			want: blob,
		},
		{
			name: "non-existing blob",
			repo: repoName,
			desc: ocispec.Descriptor{
				MediaType: "application/vnd.oci.image.layer.v1.tar",
				Digest:    "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				Size:      0,
			},
			wantErr: true,
		},
		{
			name:    "non-existing repo",
			repo:    uri.Host + "/test2",
			desc:    blobDesc,
			wantErr: true,
		},
		{
			name:    "invalid repo",
			repo:    "invalid",
			desc:    blobDesc,
			wantErr: true,
		},
		{
			name:    "blob too large",
			repo:    repoName,
			desc:    ocispec.Descriptor{Size: defaultMaxBlobBytes + 1},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.FetchBlob(ctx, tt.repo, tt.desc)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegistryStore.FetchBlob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RegistryStore.FetchBlob() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegistryStore_FetchManifest(t *testing.T) {
	manifest := &ocispec.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType:    ocispec.MediaTypeImageManifest,
		ArtifactType: "application/vnd.unknown.artifact.v1",
		Config:       ocispec.DescriptorEmptyJSON,
		Layers: []ocispec.Descriptor{
			{
				MediaType: "application/vnd.oci.image.layer.v1.tar",
				Digest:    "sha256:a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447",
				Size:      12,
				Annotations: map[string]string{
					ocispec.AnnotationTitle: "hello.txt",
				},
			},
		},
		Annotations: map[string]string{
			ocispec.AnnotationCreated: "2025-01-22T09:54:41Z",
		},
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}
	manifestDesc := content.NewDescriptorFromBytes(manifest.MediaType, manifestBytes)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected access: %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		switch r.URL.Path {
		case "/v2/test/manifests/" + manifestDesc.Digest.String():
			if accept := r.Header.Get("Accept"); !strings.Contains(accept, manifestDesc.MediaType) {
				t.Errorf("manifest not convertible: %s", accept)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", manifestDesc.MediaType)
			w.Header().Set("Docker-Content-Digest", manifestDesc.Digest.String())
			if _, err := w.Write(manifestBytes); err != nil {
				t.Errorf("failed to write %q: %v", r.URL, err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()
	uri, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("invalid test http server: %v", err)
	}
	repoName := uri.Host + "/test"
	store, err := NewRegistryStore("hello", RegistryStoreOptions{
		PlainHTTP: true,
	})
	if err != nil {
		t.Fatalf("NewRegistryStore() error = %v, want nil", err)
	}
	ctx := context.Background()

	tests := []struct {
		name    string
		repo    string
		desc    ocispec.Descriptor
		want    []byte
		wantErr bool
	}{
		{
			name: "fetch manifest",
			repo: repoName,
			desc: manifestDesc,
			want: manifestBytes,
		},
		{
			name: "non-existing manifest",
			repo: repoName,
			desc: ocispec.Descriptor{
				MediaType: "application/vnd.oci.image.manifest.v1+json",
				Digest:    "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				Size:      0,
			},
			wantErr: true,
		},
		{
			name:    "non-existing repo",
			repo:    uri.Host + "/test2",
			desc:    manifestDesc,
			wantErr: true,
		},
		{
			name:    "invalid repo",
			repo:    "invalid",
			desc:    manifestDesc,
			wantErr: true,
		},
		{
			name:    "manifest too large",
			repo:    repoName,
			desc:    ocispec.Descriptor{Size: defaultMaxManifestBytes + 1},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.FetchManifest(ctx, tt.repo, tt.desc)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegistryStore.FetchManifest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RegistryStore.FetchManifest() = %v, want %v", got, tt.want)
			}
		})
	}
}
