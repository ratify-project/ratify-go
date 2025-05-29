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

	"github.com/notaryproject/ratify-go/internal/stack"
)

type Group[Result any] struct {
	// tasks is a stack of tasks to be executed
	tasks stack.Stack[func() (Result, error)]
	// results is a slice to store results of completed tasks
	results stack.Stack[Result]

	// synchronization
	pool          chan token // shared pool for limiting concurrency
	activeTasks   int32      // atomic counter for active tasks
	taskNotifier  chan token // notify new tasks for processing
	mu            sync.Mutex // protects completion state
	completed     bool       // indicates all work is complete
	cond          *sync.Cond // condition variable for completion signaling
	completedCh   chan token // channel to signal completion
	schedulerDone chan token // channel to signal scheduler goroutine completion

	// error handling
	ctx     context.Context
	cancel  context.CancelCauseFunc
	errOnce sync.Once

	// waitCalled is an atomic flag to ensure Wait() is called only once
	waitCalled int32
}

// NewGroup creates a new group with a given size.
func NewGroup[Result any](ctx context.Context, sharedPool Pool) (*Group[Result], context.Context) {
	ctxWithCancel, cancel := context.WithCancelCause(ctx)
	g := &Group[Result]{
		ctx:           ctxWithCancel,
		cancel:        cancel,
		taskNotifier:  make(chan token, 1),
		pool:          sharedPool,
		completedCh:   make(chan token),
		schedulerDone: make(chan token),
	}
	g.cond = sync.NewCond(&g.mu)

	// main scheduler goroutine
	go func() {
		defer close(g.schedulerDone)
		for {
			select {
			case <-g.completedCh:
				return
			case <-ctx.Done():
				return
			case <-g.taskNotifier:
				select {
				case <-g.completedCh:
					return
				case <-ctx.Done():
					return
				case g.pool <- token{}:
					atomic.AddInt32(&g.activeTasks, 1)
					task, ok := g.tasks.TryPop()
					if !ok {
						// no task available, release the pool token
						<-g.pool
						atomic.AddInt32(&g.activeTasks, -1)
						continue
					}
					// check if there are more tasks to process
					if !g.tasks.IsEmpty() {
						select {
						case g.taskNotifier <- token{}:
						default:
						}
					}

					// start to process the task
					go func(f func() (Result, error)) {
						defer func() {
							<-g.pool

							// atomically decrement and check for completion
							remaining := atomic.AddInt32(&g.activeTasks, -1)
							if remaining == 0 && g.tasks.IsEmpty() {
								g.signalCompletion()
							}
						}()

						result, err := f()
						if err != nil {
							g.errOnce.Do(func() {
								g.cancel(err)
							})
							return
						}
						g.results.Push(result)
					}(task)
				}
			}
		}
	}()

	return g, ctxWithCancel
}

// Submit adds a new task to the group for execution.
func (g *Group[Result]) Submit(task func() (Result, error)) error {
	select {
	case <-g.ctx.Done():
		// check if context was canceled with cause
		if cause := context.Cause(g.ctx); cause != nil && cause != context.Canceled {
			return cause
		}
		// regular context cancellation
		return g.ctx.Err()
	default:
		g.tasks.Push(task)
	}

	select {
	case g.taskNotifier <- token{}:
	default:
	}
	return nil
}

// Wait waits for all tasks to complete and returns their results.
// If any task returns an error, it will return that error instead
// and cancel other tasks.
//
// Wait() can only be called once. Subsequent calls will return ErrWaitCalledMultipleTimes.
//
// Note: user can continue to submit tasks after calling Wait.
func (g *Group[Result]) Wait() ([]Result, error) {
	if !g.waitOnce() {
		return nil, errors.New("Wait() can only be called once on Group")
	}

	defer func() {
		close(g.completedCh)
		<-g.schedulerDone // wait for scheduler goroutine to finish
	}()

	// check early exit conditions
	if g.ctx.Err() != nil {
		// if context is already done, return the error
		if cause := context.Cause(g.ctx); cause != nil && cause != context.Canceled {
			return nil, cause
		}
		return nil, g.ctx.Err()
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// wait for all work to be completed
	for !g.completed {
		activeTasks := atomic.LoadInt32(&g.activeTasks)
		pendingTasks := !g.tasks.IsEmpty()

		// if no active tasks and no pending tasks, we're done
		if activeTasks == 0 && !pendingTasks {
			g.completed = true
			break
		}

		// if there are pending tasks but no active tasks, trigger processing
		if activeTasks == 0 && pendingTasks {
			g.mu.Unlock()
			select {
			case g.taskNotifier <- token{}:
			default:
			}
			g.mu.Lock()
		}

		// wait for completion signal or context cancellation
		// Create a goroutine to handle context cancellation
		done := make(chan struct{})
		go func() {
			select {
			case <-g.ctx.Done():
				g.mu.Lock()
				g.completed = true
				g.cond.Signal()
				g.mu.Unlock()
			case <-done:
			}
		}()

		g.cond.Wait()
		close(done)

		// check for context cancellation
		select {
		case <-g.ctx.Done():
			// check if context was canceled with cause
			if cause := context.Cause(g.ctx); cause != nil && cause != context.Canceled {
				return nil, cause
			}
			// regular context cancellation
			return nil, g.ctx.Err()
		default:
		}
	}

	return g.results.ToSlice(), nil
}

// signalCompletion safely signals that all work is complete
func (g *Group[Result]) signalCompletion() {
	g.mu.Lock()
	g.completed = true
	g.cond.Signal()
	g.mu.Unlock()
}

func (g *Group[Result]) waitOnce() bool {
	return atomic.CompareAndSwapInt32(&g.waitCalled, 0, 1)
}
