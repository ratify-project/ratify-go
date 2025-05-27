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
	// NewGroup creates a new worker pool group that shares the same
	// goroutines as the original pool.
	//
	// The new group will have its own task queue and error will
	// only cancel the new group and it's sub-pools.
	//
	// The hierarchy of group are built through the ctx passed to
	// Group.
	NewGroup(ctx context.Context) (Group, context.Context)

	// Stop stops the worker pool and all its groups.
	Stop()
}

type Group interface {
	// Submit submits a task to the pool. It will be scheduled for execution
	// when a worker is available.
	Submit(task func() error) error

	// Wait blocks until all tasks in the group are completed,
	// or until an error occurs.
	Wait() error
}
