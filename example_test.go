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

package ratify_test

import (
	"context"
	"crypto/tls"
	"net/http"
	"testing/fstest"

	"github.com/ratify-project/ratify-go"
)

func ExampleStoreMux() {
	// Create stores for demonstration.
	// Each store is an OCI image layout with a single artifact with a different
	// tag.
	ctx := context.Background()
	var stores []ratify.Store
	for _, tag := range []string{"foo", "bar", "hello", "world"} {
		fsys := fstest.MapFS{
			"blob/sha256/03c6d9dcb63b03c5489db13cedbc549f2de807f43440df549644866a24a3561b": &fstest.MapFile{
				Data: []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","artifactType":"application/vnd.unknown.artifact.v1","config":{"mediaType":"application/vnd.oci.empty.v1+json","digest":"sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a","size":2,"data":"e30="},"layers":[{"mediaType":"application/vnd.oci.empty.v1+json","digest":"sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a","size":2,"data":"e30="}],"annotations":{"org.opencontainers.image.created":"2025-01-26T07:32:59Z"}}`),
			},
			"blob/sha256/44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a": &fstest.MapFile{
				Data: []byte(`{}`),
			},
			"index.json": &fstest.MapFile{
				Data: []byte(`{"schemaVersion":2,"manifests":[{"mediaType":"application/vnd.oci.image.manifest.v1+json","digest":"sha256:03c6d9dcb63b03c5489db13cedbc549f2de807f43440df549644866a24a3561b","size":535,"annotations":{"org.opencontainers.image.created":"2025-01-26T07:32:59Z","org.opencontainers.image.ref.name":"` + tag + `"},"artifactType":"application/vnd.unknown.artifact.v1"}]}`),
			},
			"oci-layout": &fstest.MapFile{
				Data: []byte(`{"imageLayoutVersion": "1.0.0"}`),
			},
		}
		store, err := ratify.NewOCIStoreFromFS(ctx, fsys)
		if err != nil {
			panic(err)
		}
		stores = append(stores, store)
	}

	// Create a new store multiplexer
	store := ratify.NewStoreMux()

	// Register store with exact repository name match.
	store.Register("registry.example/test", stores[0])

	// Register store for an registry.
	store.Register("registry.example", stores[1])

	// Register store with wildcard DNS record.
	store.Register("*.example", stores[2])

	// Register a fallback store for unmatched requests.
	store.RegisterFallback(stores[3])

	// Reference should be resolvable by the matching store.
	_, err := store.Resolve(ctx, "registry.example/test:foo")
	if err != nil {
		panic(err)
	}
	_, err = store.Resolve(ctx, "registry.example/another-test:bar")
	if err != nil {
		panic(err)
	}
	_, err = store.Resolve(ctx, "test.example/test:hello")
	if err != nil {
		panic(err)
	}
	_, err = store.Resolve(ctx, "another.registry.example/test:world")
	if err != nil {
		panic(err)
	}
	// Output:
}

// ExampleStoreMux_mixRegistryStore demonstrates how to access different
// registries with different network configurations using a single store by
// multiplexing.
func ExampleStoreMux_mixRegistryStore() {
	// Create a new store multiplexer
	mux := ratify.NewStoreMux()

	// Create a global registry store with default options.
	// Developers should replace the default options with actual configuration.
	store := ratify.NewRegistryStore(ratify.RegistryStoreOptions{})
	if err := mux.RegisterFallback(store); err != nil {
		panic(err)
	}

	// Create a registry store for local registry.
	// A local registry is accessed over plain HTTP unlike the global registry.
	store = ratify.NewRegistryStore(ratify.RegistryStoreOptions{
		PlainHTTP: true,
	})
	if err := mux.Register("localhost:5000", store); err != nil {
		panic(err)
	}

	// Create a registry store with client certificate authentication.
	store = ratify.NewRegistryStore(ratify.RegistryStoreOptions{
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					Certificates: []tls.Certificate{
						// add client certificate with private key
					},
				},
			},
		},
	})
	if err := mux.Register("private.registry.example", store); err != nil {
		panic(err)
	}

	// Create a registry store for an insecure registry.
	store = ratify.NewRegistryStore(ratify.RegistryStoreOptions{
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
	})
	if err := mux.Register("insecure.registry.example", store); err != nil {
		panic(err)
	}
	// Output:
}
