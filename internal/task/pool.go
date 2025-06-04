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
	"context"

	"github.com/notaryproject/ratify-go/internal/workerpool"
)

type TaskPool struct {
	wg         WaitGroup
	tasks      *TaskStack[func() error]
	workerpool *workerpool.Pool[any]
}

func NewTaskPool(ctx context.Context, size int) (*TaskPool, context.Context) {
	pool, ctx := workerpool.New[any](ctx, size)
	p := &TaskPool{
		wg:         WaitGroup{},
		tasks:      NewTaskStack[func() error](size),
		workerpool: pool,
	}

	// Start a goroutine to process tasks from the stack
	go func() {
		for {
			select {
			case <-p.wg.Complete():
				return
			case task, ok := <-p.tasks.PopChannel():
				if !ok {
					return
				}
				if err := pool.Go(func() (any, error) {
					defer p.wg.Done()
					return nil, task()
				}); err != nil {
					return
				}
			}
		}
	}()

	return p, ctx
}

func (p *TaskPool) Submit(task func() error) {
	p.wg.Add(1)
	p.tasks.Push(task)
}

func (p *TaskPool) Wait() error {
	p.wg.Wait()

	if _, err := p.workerpool.Wait(); err != nil {
		return err
	}
	return nil
}
