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

type resolver interface {
	Resolve(ctx context.Context, reference string) (ocispec.Descriptor, error)
}

type tagResolver struct {
	Stores []Store
}

func (r *tagResolver) Resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	for _, store := range r.Stores {
		desc, err := store.Resolve(ctx, reference)
		if err == nil {
			return desc, nil
		}
	}
	return ocispec.Descriptor{}, fmt.Errorf("unable to resolve reference: %s", reference)
}
