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

package worker

type Group[T any] interface {
	// Submit submits a task to the pool. It will be scheduled for execution
	// when a worker is available. The task returns a value of type T and an error.
	Submit(task func() (T, error)) error

	// Wait blocks until all tasks in the group are completed,
	// or until an error occurs. Returns all results in a slice.
	Wait() ([]T, error)
}
