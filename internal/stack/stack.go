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

// Stack is a generic stack implementation.
type Stack[T any] []T

// Push pushes keys to the stack.
func (s *Stack[T]) Push(keys ...T) {
	*s = append(*s, keys...)
}

// Pop pops keys from the stack.
// Panics if the stack is empty.
func (s *Stack[T]) Pop() T {
	if len(*s) == 0 {
		panic("pop from empty stack")
	}
	t := (*s)[len(*s)-1]
	*s = (*s)[:len(*s)-1]
	return t
}

// IsEmpty checks if the stack is empty.
func (s *Stack[T]) Len() int {
	return len(*s)
}
