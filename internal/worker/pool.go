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

type token struct{}

type pool struct {
	semaphore chan token
}

func NewPool(size int) Pool {
	return &pool{
		semaphore: make(chan token, size),
	}
}

func (p *pool) NewGroup(ctx context.Context) (Group, context.Context) {
	return newGroup(ctx, p.semaphore)
}

func (p *pool) Stop() {
	// Closing the semaphore channel will stop all groups from accepting new tasks.
	close(p.semaphore)
}
