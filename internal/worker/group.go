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
	"sync"
	"sync/atomic"

	"github.com/notaryproject/ratify-go/internal/stack"
)

type Group[Result any] struct {
	// tasks is a stack of tasks to be executed
	tasks stack.Stack[func() (Result, error)]

	// synchronization
	notifier    chan token
	pool        chan token
	activeTasks int32      // atomic counter for active tasks
	mu          sync.Mutex // protects completion state
	completed   bool       // indicates all work is complete
	cond        *sync.Cond // condition variable for completion signaling

	// error handling
	ctx     context.Context
	cancel  context.CancelCauseFunc
	errOnce sync.Once
	err     error

	// results
	resultsMu sync.Mutex
	results   []Result
}

// NewGroup creates a new group with a given size.
func NewGroup[Result any](ctx context.Context, sharedPool Pool) (*Group[Result], context.Context) {
	ctxWithCancel, cancel := context.WithCancelCause(ctx)
	g := &Group[Result]{
		ctx:      ctxWithCancel,
		cancel:   cancel,
		notifier: make(chan token, 1),
		pool:     sharedPool,
	}
	g.cond = sync.NewCond(&g.mu)

	go func() {
		// Main worker goroutine

		for {
			select {
			case <-ctx.Done():
				return
			case <-g.notifier:
				// Check if we need to process tasks
				if g.tasks.IsEmpty() {
					continue
				}

				select {
				case <-ctx.Done():
					return
				case g.pool <- token{}:
					atomic.AddInt32(&g.activeTasks, 1)
					task, ok := g.tasks.TryPop()
					if !ok {
						// No task available, release the pool token
						<-g.pool
						atomic.AddInt32(&g.activeTasks, -1)
						continue
					}

					go func(taskFunc func() (Result, error)) {
						defer func() {
							<-g.pool

							// Atomically decrement and check for completion
							remaining := atomic.AddInt32(&g.activeTasks, -1)
							if remaining == 0 && g.tasks.IsEmpty() {
								g.signalCompletion()
							}
						}()

						result, err := taskFunc()
						if err != nil {
							g.errOnce.Do(func() {
								g.err = err
								g.cancel(err)
							})
						} else {
							g.resultsMu.Lock()
							g.results = append(g.results, result)
							g.resultsMu.Unlock()
						}
					}(task)

					// Check if there are more tasks to process
					if !g.tasks.IsEmpty() {
						select {
						case g.notifier <- token{}:
						default:
						}
					}
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
		// Check if it was our internal cancellation due to error
		if g.err != nil {
			return g.err
		}
		// Check if context was canceled with cause
		if cause := context.Cause(g.ctx); cause != nil && cause != context.Canceled {
			return cause
		}
		// Regular context cancellation
		return g.ctx.Err()
	default:
		g.tasks.Push(task)
	}

	select {
	case g.notifier <- token{}:
	default:
	}
	return nil
}

// Wait waits for all tasks to complete and returns their results.
// If any task returns an error, it will return that error instead
// and cancel other tasks.
//
// Note: user can continue to submit tasks after calling Wait.
func (g *Group[Result]) Wait() ([]Result, error) {
	// Check early exit conditions
	if g.err != nil {
		return nil, g.err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	// Wait for all work to be completed
	for !g.completed {
		activeTasks := atomic.LoadInt32(&g.activeTasks)
		pendingTasks := !g.tasks.IsEmpty()

		// If no active tasks and no pending tasks, we're done
		if activeTasks == 0 && !pendingTasks {
			g.completed = true
			break
		}

		// If there are pending tasks but no active tasks, trigger processing
		if activeTasks == 0 && pendingTasks {
			g.mu.Unlock()
			select {
			case g.notifier <- token{}:
			default:
			}
			g.mu.Lock()
		}

		// Wait for completion signal or context cancellation
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

		// Check for context cancellation
		select {
		case <-g.ctx.Done():
			// Check if it was our internal cancellation due to error
			if g.err != nil {
				return nil, g.err
			}
			// Check if context was canceled with cause
			if cause := context.Cause(g.ctx); cause != nil && cause != context.Canceled {
				return nil, cause
			}
			// Regular context cancellation
			return nil, g.ctx.Err()
		default:
		}
	}

	// Double-check for errors after completion
	if g.err != nil {
		return nil, g.err
	}

	g.resultsMu.Lock()
	defer g.resultsMu.Unlock()
	// Return a copy to avoid race conditions
	resultsCopy := make([]Result, len(g.results))
	copy(resultsCopy, g.results)
	return resultsCopy, nil
}

// signalCompletion safely signals that all work is complete
func (g *Group[Result]) signalCompletion() {
	g.mu.Lock()
	g.completed = true
	g.cond.Signal()
	g.mu.Unlock()
}
