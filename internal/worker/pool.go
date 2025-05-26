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
	tasks stack.Stack[func() error]

	// synchronization
	hasTask   chan token
	semaphore chan token
	wg        sync.WaitGroup

	// error handling
	cancel  context.CancelCauseFunc
	errOnce sync.Once
	err     error
}

// NewPool creates a new worker pool.
func NewPool(ctx context.Context, size int) (*pool, context.Context) {
	ctxWithCancel, cancel := context.WithCancelCause(ctx)
	p := &pool{
		cancel:    cancel,
		hasTask:   make(chan token, 1),
		semaphore: make(chan token, size),
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(p.semaphore)
				return
			case <-p.hasTask:
				select {
				case <-ctx.Done():
					close(p.semaphore)
					return
				case p.semaphore <- token{}:
					task := p.tasks.Pop()
					p.wg.Add(1)
					go func() {
						defer func() {
							p.wg.Done()
							<-p.semaphore
						}()

						if task != nil {
							if err := task(); err != nil {
								p.errOnce.Do(func() {
									p.err = err
									// Cancel the context with the error
									p.cancel(err)
								})
							}
						}
					}()
					if p.tasks.Len() > 0 {
						// Notify that there is another task available
						select {
						case p.hasTask <- token{}:
						default:
						}
					}
				}
			}
		}
	}()

	return p, ctxWithCancel
}

func (p *pool) Submit(task func() error) error {
	p.tasks.Push(task)
	// notify that there is a new task available
	select {
	case p.hasTask <- token{}:
	default:
	}
	return nil
}

func (p *pool) Wait() error {
	for {
		p.wg.Wait()
		if p.err != nil {
			return p.err
		}
		if p.tasks.Len() == 0 {
			break
		}
	}
	return nil
}
