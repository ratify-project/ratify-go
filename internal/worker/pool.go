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

type token struct{}

type pool struct {
	// tasks is a stack of tasks to be executed
	tasks stack.Stack[func() error]

	// synchronization
	//
	// notifier has a single token to indicate that there is a
	// task available
	notifier chan token
	// semaphore is used to limit the number of concurrent tasks
	semaphore chan token
	// wg is used to wait for all tasks to complete
	wg sync.WaitGroup

	// error handling
	cancel  context.CancelCauseFunc
	errOnce sync.Once
	err     error
}

func NewPool(ctx context.Context, size int) (*pool, context.Context) {
	return newPool(ctx, make(chan token, size))
}

func newPool(ctx context.Context, semaphore chan token) (*pool, context.Context) {
	ctxWithCancel, cancel := context.WithCancelCause(ctx)
	g := &pool{
		cancel:    cancel,
		notifier:  make(chan token, 1),
		semaphore: semaphore,
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(g.semaphore)
				return
			case <-g.notifier:
				select {
				case <-ctx.Done():
					close(g.semaphore)
					return
				case g.semaphore <- token{}:
					task := g.tasks.Pop()
					g.wg.Add(1)
					go func() {
						defer func() {
							g.wg.Done()
							<-g.semaphore
						}()

						if task != nil {
							if err := task(); err != nil {
								g.errOnce.Do(func() {
									g.err = err
									// Cancel the context with the error
									g.cancel(err)
								})
							}
						}
					}()
					if g.tasks.Len() > 0 {
						// Notify that there is another task available
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

func (g *pool) Submit(task func() error) error {
	g.tasks.Push(task)
	// notify that there is a new task available
	select {
	case g.notifier <- token{}:
	default:
	}
	return nil
}

func (g *pool) Wait() error {
	for {
		g.wg.Wait()
		if g.err != nil {
			return g.err
		}
		if g.tasks.Len() == 0 {
			break
		}
	}
	return nil
}

func (g *pool) SharedPool(ctx context.Context) (Pool, context.Context) {
	return newPool(ctx, g.semaphore)
}
