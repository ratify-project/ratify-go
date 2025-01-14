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

// task is a struct that represents a task that verifies an artifact by
// the executor.
type task struct {
	// artifact is the digested reference of the referrer artifact that will be
	// verified. Required.
	artifact string

	// registry is the registry where the artifact is stored. Required.
	registry string

	// repo is the repository where the artifact is stored. Required.
	repo string

	// store is the store that stores the artifacts. Required.
	store Store

	// subjectReport is the report of the subject artifact. Optional.
	subjectReport *ValidationReport
}
