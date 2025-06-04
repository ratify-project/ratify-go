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

package task

import (
	"sync"

	"github.com/notaryproject/ratify-go/internal/stack"
)

// TaskStack is a stack-based task queue that allows tasks to be pushed onto a stack
// and then popped through a channel.
type TaskStack[Task any] struct {
	mu    sync.Mutex
	ch    chan Task
	stack stack.Stack[Task]

	// drainingOnce is used to ensure that the drainStack goroutine is started only once
	drainingOnce sync.Once
}

// NewTaskStack creates a new TaskStack with the specified channel buffer size.
func NewTaskStack[Task any](bufferSize int) *TaskStack[Task] {
	return &TaskStack[Task]{
		ch: make(chan Task, bufferSize),
	}
}

func (s *TaskStack[Task]) Push(task Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.stack.Push(task)

	s.drainingOnce.Do(func() {
		go s.drainStack()
	})
}

// drainStack continuously moves tasks from the stack to the channel
func (s *TaskStack[Task]) drainStack() {
	for {
		s.mu.Lock()
		if s.stack.Len() == 0 {
			// reset to allow future pushes to restart draining
			s.drainingOnce = sync.Once{}
			s.mu.Unlock()
			return
		}

		task := s.stack.Pop()
		s.mu.Unlock()

		s.ch <- task
	}
}

// PopChannel returns a channel that can be used to receive tasks from the stack.
func (s *TaskStack[Task]) PopChannel() <-chan Task {
	return s.ch
}
