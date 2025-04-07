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

package set

import (
	"testing"
)

func TestNewSet(t *testing.T) {
	s := New(1, 2, 3)
	if len(s) != 3 {
		t.Errorf("expected set length 3, got %d", len(s))
	}
	for _, v := range []int{1, 2, 3} {
		if !s.Contains(v) {
			t.Errorf("expected set to contain %d", v)
		}
	}
}

func TestSetContains(t *testing.T) {
	s := New("a", "b", "c")
	tests := []struct {
		elem     string
		expected bool
	}{
		{"a", true},
		{"b", true},
		{"x", false},
	}

	for _, tt := range tests {
		if got := s.Contains(tt.elem); got != tt.expected {
			t.Errorf("Contains(%q) = %v; want %v", tt.elem, got, tt.expected)
		}
	}
}

func TestSetAdd(t *testing.T) {
	s1 := New(1, 2)
	s2 := New(2, 3, 4)

	s1.Union(s2)

	expectedElems := []int{1, 2, 3, 4}
	for _, v := range expectedElems {
		if !s1.Contains(v) {
			t.Errorf("expected set to contain %d after Add", v)
		}
	}

	if len(s1) != 4 {
		t.Errorf("expected set length 4 after Add, got %d", len(s1))
	}
}

func TestEmptySet(t *testing.T) {
	s := New[int]()
	if len(s) != 0 {
		t.Errorf("expected empty set length 0, got %d", len(s))
	}
	if s.Contains(1) {
		t.Errorf("expected empty set not to contain any elements")
	}
}

func TestAddEmptySet(t *testing.T) {
	s1 := New(1, 2)
	s2 := New[int]()

	s1.Union(s2)

	if len(s1) != 2 {
		t.Errorf("expected set length 2 after adding empty set, got %d", len(s1))
	}
}
