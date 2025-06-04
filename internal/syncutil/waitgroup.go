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

package syncutil

import "sync"

// waitGroup is a wrapper around sync.WaitGroup that provides a channel
// to signal when all tasks are complete.
type waitGroup struct {
	sync.WaitGroup

	doneOnce sync.Once
	done     chan struct{}
}

// Complete returns a channel that will be closed when all tasks in the wait group are complete.
func (wg *waitGroup) Complete() <-chan struct{} {
	wg.doneOnce.Do(func() {
		wg.done = make(chan struct{})
		go func() {
			wg.Wait()
			close(wg.done)
		}()
	})

	return wg.done
}
