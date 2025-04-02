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

import "maps"

// Set represents a generic set data structure using a map with empty structs.
type Set[E comparable] map[E]struct{}

// NewSet creates and returns a new Set initialized with the provided elements.
func NewSet[E comparable](elems ...E) *Set[E] {
	s := &Set[E]{}
	for _, e := range elems {
		(*s)[e] = struct{}{}
	}
	return s
}

// Contains checks if the set contains the specified element.
func (s *Set[E]) Contains(v E) bool {
	_, ok := (*s)[v]
	return ok
}

// Add inserts one or more elements into the set.
func (s *Set[E]) Add(elems ...E) {
	for _, e := range elems {
		(*s)[e] = struct{}{}
	}
}

// Copy copies all elements from the source set into the current set.
func (s *Set[E]) Copy(src *Set[E]) {
	maps.Copy(*s, *src)
}