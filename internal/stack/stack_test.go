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

import "testing"

func TestStack(t *testing.T) {
	s := Stack[int]{}
	if s.Len() != 0 {
		t.Errorf("Expected stack to be empty")
	}

	s.Push(1, 2)
	x := s.Pop()
	if x != 2 {
		t.Errorf("Expected key to be 2")
	}

	x = s.Pop()
	if x != 1 {
		t.Errorf("Expected key to be 1")
	}

	if s.Len() != 0 {
		t.Errorf("Expected stack to be empty")
	}
}

func TestStack_PopEmptyStack(t *testing.T) {
	s := Stack[int]{}

	// Pop from empty stack should return zero value
	x := s.Pop()
	if x != 0 {
		t.Errorf("Expected zero value (0) from empty stack, got %d", x)
	}

	// TryPop should return false for empty stack
	val, ok := s.TryPop()
	if ok {
		t.Errorf("Expected TryPop to return false for empty stack")
	}
	if val != 0 {
		t.Errorf("Expected zero value (0) from TryPop on empty stack, got %d", val)
	}
}

func TestStack_TryPop(t *testing.T) {
	s := Stack[int]{}
	s.Push(42)

	// TryPop should return true and the value for non-empty stack
	val, ok := s.TryPop()
	if !ok {
		t.Errorf("Expected TryPop to return true for non-empty stack")
	}
	if val != 42 {
		t.Errorf("Expected value 42 from TryPop, got %d", val)
	}

	// Stack should now be empty
	if !s.IsEmpty() {
		t.Errorf("Expected stack to be empty after TryPop")
	}
}

func TestStack_IsEmpty(t *testing.T) {
	s := Stack[int]{}

	if !s.IsEmpty() {
		t.Errorf("Expected new stack to be empty")
	}

	s.Push(1)
	if s.IsEmpty() {
		t.Errorf("Expected stack with item to not be empty")
	}

	s.Pop()
	if !s.IsEmpty() {
		t.Errorf("Expected stack to be empty after popping last item")
	}
}
