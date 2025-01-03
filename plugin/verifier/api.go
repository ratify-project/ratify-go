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

package verifier

import (
	"context"

	oci "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/ratify-project/ratify-go/internal/common"
	"github.com/ratify-project/ratify-go/plugin/store"
)

// Verifier is an interface that defines methods to verify an artifact associated
// with a subject.
type Verifier interface {
	// Name returns the name of the verifier
	Name() string

	// Type returns the type name of the verifier
	Type() string

	// CanVerify returns if the verifier can verify the given reference
	CanVerify(ctx context.Context, referrerDescriptor oci.Descriptor) bool

	// Verify verifies the given reference of a subject and returns the verification result
	Verify(ctx context.Context,
		subjectReference common.Reference,
		referrerDescriptor oci.Descriptor,
		referrerStore store.ReferrerStore) (VerifierResult, error)
}
