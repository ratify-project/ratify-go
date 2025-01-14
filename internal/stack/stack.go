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
type Stack[T any] struct {
	keys []T
}

// NewStack creates a new stack.
func NewStack[T any]() *Stack[T] {
	return &Stack[T]{}
}

// Push pushes keys to the stack.
func (stack *Stack[T]) Push(keys ...T) {
	stack.keys = append(stack.keys, keys...)
}

// Pop pops keys from the stack.
func (stack *Stack[T]) Pop() (T, bool) {
	var x T
	if len(stack.keys) > 0 {
		x := stack.keys[len(stack.keys)-1]
		stack.keys = stack.keys[:len(stack.keys)-1]
		return x, true
	}
	return x, false
}

// IsEmpty checks if the stack is empty.
func (stack *Stack[T]) IsEmpty() bool {
	return len(stack.keys) == 0
}
