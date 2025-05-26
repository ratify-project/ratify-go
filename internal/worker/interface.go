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

import "context"

type Pool interface {
	Group

	// NewGroup creates a new Group for managing a set of tasks.
	NewGroup(ctx context.Context) (Group, context.Context)
}

type Group interface {
	// Submit submits a task to the group.
	Submit(task func() error) error

	// Wait blocks until all tasks in the group are completed,
	// or until an error occurs.
	Wait() error
}
