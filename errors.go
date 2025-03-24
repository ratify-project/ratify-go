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

package ratify

import "errors"

// ErrSubjectPruned is returned when the evaluator does not need given subject
// to be verified to make a decision by [Evaluator.Pruned].
var ErrSubjectPruned = errors.New("evaluator sub-graph is pruned for the subject")
