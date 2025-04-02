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

func TestSetContains(t *testing.T) {
	s := NewSet("a", "b", "c")
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
			t.Errorf("Contains(%q) = %v, want %v", tt.elem, got, tt.expected)
		}
	}
}

func TestSetAdd(t *testing.T) {
	s := NewSet[int]()
	s.Add(1, 2, 3)
	if len(*s) != 3 {
		t.Errorf("Add() expected length 3, got %d", len(*s))
	}
	s.Add(3, 4)
	if len(*s) != 4 {
		t.Errorf("Add() expected length 4 after adding duplicate, got %d", len(*s))
	}
	if !s.Contains(4) {
		t.Errorf("Add() missing element 4 after adding")
	}
}

func TestSetCopy(t *testing.T) {
	src := NewSet(1, 2, 3)
	dest := NewSet[int]()
	dest.Copy(src)

	if len(*dest) != len(*src) {
		t.Errorf("Copy() expected length %d, got %d", len(*src), len(*dest))
	}

	for k := range *src {
		if !dest.Contains(k) {
			t.Errorf("Copy() missing element %d in destination set", k)
		}
	}
}

func TestSetEmpty(t *testing.T) {
	s := NewSet[int]()
	if len(*s) != 0 {
		t.Errorf("NewSet() expected empty set, got length %d", len(*s))
	}
	if s.Contains(1) {
		t.Errorf("Empty set should not contain any elements")
	}
}

func TestSetAddDuplicate(t *testing.T) {
	s := NewSet("a")
	s.Add("a", "a")
	if len(*s) != 1 {
		t.Errorf("Add() expected length 1 after adding duplicates, got %d", len(*s))
	}
}
