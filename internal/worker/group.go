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

	"github.com/notaryproject/ratify-go/internal/stack"
)

type group[T any] struct {
	// tasks is a stack of tasks to be executed
	tasks stack.Stack[func() (T, error)]

	// synchronization
	notifier chan token
	pool     chan token
	wg       sync.WaitGroup

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
func NewGroup[T any](ctx context.Context, pool chan token) (*group[T], context.Context) {
	return newGroup[T](ctx, pool)
}

func newGroup[T any](ctx context.Context, pool chan token) (*group[T], context.Context) {
	ctxWithCancel, cancel := context.WithCancelCause(ctx)
	g := &group[T]{
		ctx:      ctxWithCancel,
		cancel:   cancel,
		notifier: make(chan token, 1),
		pool:     pool,
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
					g.wg.Add(1)
					task := g.tasks.Pop()
					go func() {
						defer func() {
							g.wg.Done()
							<-g.pool
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
	for {
		g.wg.Wait()
		if g.err != nil {
			return nil, g.err
		}
		if g.tasks.Len() == 0 {
			break
		}
	}
	g.resultsMu.Lock()
	defer g.resultsMu.Unlock()
	// Return a copy to avoid race conditions
	resultsCopy := make([]T, len(g.results))
	copy(resultsCopy, g.results)
	return resultsCopy, nil
}
