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

package stack

import "sync"

// Stack is a concurrent-safe generic stack implementation.
type Stack[T any] struct {
	items []T
	mu    sync.RWMutex
}

// Push pushes keys to the stack.
func (s *Stack[T]) Push(keys ...T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, keys...)
}

// Pop pops a key from the stack.
func (s *Stack[T]) Pop() T {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.items) == 0 {
		var zero T
		return zero
	}

	t := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return t
}

// TryPop attempts to pop an item from the stack.
// Returns the popped item and true if successful, zero value and false if empty.
func (s *Stack[T]) TryPop() (T, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.items) == 0 {
		var zero T
		return zero, false
	}

	t := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return t, true
}

// Len returns the number of elements in the stack.
func (s *Stack[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// IsEmpty returns true if the stack is empty.
func (s *Stack[T]) IsEmpty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items) == 0
}
