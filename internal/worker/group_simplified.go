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

package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/notaryproject/ratify-go/internal/stack"
)

// TaskState manages the execution state of tasks
type TaskState struct {
	activeTasks int32
	mu          sync.RWMutex
	completed   bool
}

func (ts *TaskState) IncrementActive() {
	atomic.AddInt32(&ts.activeTasks, 1)
}

func (ts *TaskState) DecrementActive() int32 {
	return atomic.AddInt32(&ts.activeTasks, -1)
}

func (ts *TaskState) ActiveCount() int32 {
	return atomic.LoadInt32(&ts.activeTasks)
}

func (ts *TaskState) SetCompleted() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.completed = true
}

func (ts *TaskState) IsCompleted() bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.completed
}

// Scheduler handles task scheduling and execution
type Scheduler[Result any] struct {
	tasks   stack.Stack[func() (Result, error)]
	results stack.Stack[Result]
	pool    Pool
	state   *TaskState

	ctx    context.Context
	cancel context.CancelCauseFunc

	taskCh chan struct{}
	done   chan struct{}
	once   sync.Once
}

func NewScheduler[Result any](ctx context.Context, pool Pool) *Scheduler[Result] {
	ctxWithCancel, cancel := context.WithCancelCause(ctx)
	return &Scheduler[Result]{
		pool:   pool,
		state:  &TaskState{},
		ctx:    ctxWithCancel,
		cancel: cancel,
		taskCh: make(chan struct{}, 1),
		done:   make(chan struct{}),
	}
}

func (s *Scheduler[Result]) Submit(task func() (Result, error)) error {
	select {
	case <-s.ctx.Done():
		return context.Cause(s.ctx)
	default:
		s.tasks.Push(task)
		s.notifyNewTask()
		return nil
	}
}

func (s *Scheduler[Result]) notifyNewTask() {
	select {
	case s.taskCh <- struct{}{}:
	default:
	}
}

func (s *Scheduler[Result]) Start() {
	go s.run()
}

func (s *Scheduler[Result]) run() {
	defer close(s.done)

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.taskCh:
			s.processAvailableTasks()
		}
	}
}

func (s *Scheduler[Result]) processAvailableTasks() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case s.pool <- ticket{}:
			task, ok := s.tasks.TryPop()
			if !ok {
				<-s.pool // release token if no task
				return
			}

			s.state.IncrementActive()
			go s.executeTask(task)

			// Continue if more tasks available
			if s.tasks.IsEmpty() {
				return
			}
		default:
			return // pool is full
		}
	}
}

func (s *Scheduler[Result]) executeTask(task func() (Result, error)) {
	defer func() {
		<-s.pool
		remaining := s.state.DecrementActive()

		// Check for completion
		if remaining == 0 && s.tasks.IsEmpty() {
			s.state.SetCompleted()
		}
	}()

	result, err := task()
	if err != nil {
		s.once.Do(func() {
			s.cancel(err)
		})
		return
	}

	s.results.Push(result)
}

func (s *Scheduler[Result]) Wait() ([]Result, error) {
	defer func() {
		s.cancel(context.Canceled)
		<-s.done
	}()

	// Wait for completion using polling with backoff
	for !s.isWorkComplete() {
		select {
		case <-s.ctx.Done():
			if cause := context.Cause(s.ctx); cause != context.Canceled {
				return nil, cause
			}
			return nil, s.ctx.Err()
		default:
			// Trigger task processing if needed
			if s.state.ActiveCount() == 0 && !s.tasks.IsEmpty() {
				s.notifyNewTask()
			}

			// Brief sleep to avoid busy waiting
			select {
			case <-s.ctx.Done():
				continue
			case <-time.After(time.Millisecond):
			}
		}
	}

	return s.results.ToSlice(), nil
}

func (s *Scheduler[Result]) isWorkComplete() bool {
	return s.state.IsCompleted() || (s.state.ActiveCount() == 0 && s.tasks.IsEmpty())
}

// SimplifiedGroup provides a cleaner interface
type SimplifiedGroup[Result any] struct {
	scheduler *Scheduler[Result]
	waitOnce  sync.Once
	waited    bool
	mu        sync.Mutex
}

func NewSimplifiedGroup[Result any](ctx context.Context, sharedPool Pool) (*SimplifiedGroup[Result], context.Context) {
	scheduler := NewScheduler[Result](ctx, sharedPool)
	scheduler.Start()

	return &SimplifiedGroup[Result]{
		scheduler: scheduler,
	}, scheduler.ctx
}

func (g *SimplifiedGroup[Result]) Submit(task func() (Result, error)) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.waited {
		return errors.New("cannot submit tasks after Wait() has been called")
	}

	return g.scheduler.Submit(task)
}

func (g *SimplifiedGroup[Result]) Wait() ([]Result, error) {
	var results []Result
	var err error

	g.waitOnce.Do(func() {
		g.mu.Lock()
		g.waited = true
		g.mu.Unlock()

		results, err = g.scheduler.Wait()
	})

	if results == nil && err == nil {
		return nil, errors.New("Wait() can only be called once on Group")
	}

	return results, err
}
