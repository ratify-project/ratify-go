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

type group[T any] struct {
	// tasks is a stack of tasks to be executed
	tasks stack.Stack[func() (T, error)]

	// synchronization
	notifier    chan token
	pool        chan token
	activeTasks int32         // atomic counter for active tasks
	finished    chan struct{} // signals when all work is complete

	// error handling
	ctx     context.Context
	cancel  context.CancelCauseFunc
	errOnce sync.Once
	err     error

	// results
	resultsMu sync.Mutex
	results   []T
}

// NewGroup creates a new group with a given size.
func NewGroup[T any](ctx context.Context, pool Pool) (*group[T], context.Context) {
	return newGroup[T](ctx, pool)
}

func newGroup[T any](ctx context.Context, pool Pool) (*group[T], context.Context) {
	ctxWithCancel, cancel := context.WithCancelCause(ctx)
	g := &group[T]{
		ctx:      ctxWithCancel,
		cancel:   cancel,
		notifier: make(chan token, 1),
		pool:     pool,
		finished: make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-g.notifier:
				select {
				case <-ctx.Done():
					return
				case g.pool <- token{}:
					atomic.AddInt32(&g.activeTasks, 1)
					task := g.tasks.Pop()
					go func() {
						defer func() {
							<-g.pool

							// Atomically decrement and check for completion
							remaining := atomic.AddInt32(&g.activeTasks, -1)
							if remaining == 0 && g.tasks.Len() == 0 {
								select {
								case g.finished <- struct{}{}:
								default:
								}
							}
						}()

						if task != nil {
							result, err := task()
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
						}
					}()
					if g.tasks.Len() > 0 {
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

func (g *group[T]) Submit(task func() (T, error)) error {
	select {
	case <-g.ctx.Done():
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

func (g *group[T]) Wait() ([]T, error) {
	// Wait for all work to be completed
	for {
		// Check if we have any active tasks or pending tasks
		if atomic.LoadInt32(&g.activeTasks) == 0 && g.tasks.Len() == 0 {
			break
		}

		// If there are pending tasks but no active tasks, trigger processing
		if atomic.LoadInt32(&g.activeTasks) == 0 && g.tasks.Len() > 0 {
			select {
			case g.notifier <- token{}:
			default:
			}
		}

		// Wait for completion signal or context cancellation
		select {
		case <-g.finished:
			// Continue the loop to double-check the condition
		case <-g.ctx.Done():
			if err := g.ctx.Err(); err == context.Canceled {
				// If the context was canceled, we should return an error
				return nil, context.Cause(g.ctx)
			} else {
				return nil, err
			}
		}
	}

	if g.err != nil {
		return nil, g.err
	}

	g.resultsMu.Lock()
	defer g.resultsMu.Unlock()
	// Return a copy to avoid race conditions
	resultsCopy := make([]T, len(g.results))
	copy(resultsCopy, g.results)
	return resultsCopy, nil
}
