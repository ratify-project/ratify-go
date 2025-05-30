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
)

// SimpleGroup is a much simpler version that uses sync.WaitGroup
// and eliminates most of the complexity while keeping the same interface
type SimpleGroup[Result any] struct {
	pool   Pool
	ctx    context.Context
	cancel context.CancelCauseFunc

	completed chan ticket
	wg        sync.WaitGroup
	errOnce   sync.Once

	waitCalled int32

	// results stores the results of completed tasks
	results   []Result
	resultsMu sync.Mutex
}

func NewSimpleGroup[Result any](ctx context.Context, sharedPool Pool) (*SimpleGroup[Result], context.Context) {
	ctxWithCancel, cancel := context.WithCancelCause(ctx)
	return &SimpleGroup[Result]{
		pool:      sharedPool,
		ctx:       ctxWithCancel,
		cancel:    cancel,
		completed: make(chan ticket),
	}, ctxWithCancel
}

func (g *SimpleGroup[Result]) Submit(task func() (Result, error)) error {
	select {
	case <-g.ctx.Done():
		if cause := context.Cause(g.ctx); cause != nil && cause != context.Canceled {
			return cause
		}
		return g.ctx.Err()
	case <-g.completed:
		// group has been completed, no more tasks can be submitted
		return errors.New("group has already been completed")
	case g.pool <- ticket{}:
		// acquired a slot in the pool
	}

	g.wg.Add(1)
	go func() {
		defer func() {
			<-g.pool // Release pool slot
			g.wg.Done()
		}()

		// execute task
		result, err := task()
		if err != nil {
			g.errOnce.Do(func() {
				g.cancel(err)
			})
			return
		}

		g.resultsMu.Lock()
		g.results = append(g.results, result)
		g.resultsMu.Unlock()
	}()

	return nil
}

func (g *SimpleGroup[Result]) Wait() ([]Result, error) {
	if !g.waitOnce() {
		return nil, errors.New("Wait() can only be called once on SimpleGroup")
	}

	// convert g.wg.Wait() to a done chan to avoid blocking
	go func() {
		g.wg.Wait()
		close(g.completed)
	}()

	select {
	case <-g.completed:
		// all tasks completed normally
	case <-g.ctx.Done():
		// context was cancelled, then wait for cleanup
		<-g.completed
	}

	// Check for errors
	if cause := context.Cause(g.ctx); cause != nil && cause != context.Canceled {
		return g.results, cause
	}

	if g.ctx.Err() != nil {
		return g.results, g.ctx.Err()
	}

	return g.results, nil
}

func (g *SimpleGroup[Result]) waitOnce() bool {
	return atomic.CompareAndSwapInt32(&g.waitCalled, 0, 1)
}
