# Copyright The Ratify Authors.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

.PHONY: all
all: test

.PHONY: test
test: check-line-endings ## run unit tests
	go test -race -v -coverprofile=coverage.txt -covermode=atomic ./...

.PHONY: clean
clean:
	git status --ignored --short | grep '^!! ' | sed 's/!! //' | xargs rm -rf

.PHONY: check-line-endings
check-line-endings: ## check line endings
	! find . -name "*.go" -type f -exec file "{}" ";" | grep CRLF
	! find -name "*.sh" -type f -exec file "{}" ";" | grep CRLF

.PHONY: fix-line-endings
fix-line-endings: ## fix line endings
	find . -type f -name "*.go" -exec sed -i -e "s/\r//g" {} +
