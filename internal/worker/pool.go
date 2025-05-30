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

type slot struct{}

// PoolSlots is a channel-based semaphore that limits the
// number of concurrent task in a [Pool].
type PoolSlots chan slot

// Pool is a worker pool that allows concurrent execution of tasks
type Pool[Result any] struct {
	// ctx is the context for the pool, which can be cancelled
	ctx    context.Context
	cancel context.CancelCauseFunc

	// poolSlots is a channel that limits the number of concurrent tasks
	poolSlots     PoolSlots
	dedicatedPool bool

	// done is a channel that signals when all tasks are done
	done    chan struct{}
	wg      sync.WaitGroup
	errOnce sync.Once

	// results stores the results of completed tasks
	results   []Result
	resultsMu sync.Mutex

	// panic recovery
	panicValue any
	panicOnce  sync.Once

	// hasWaited is used to ensure Wait() can only be called once
	hasWaited atomic.Bool
}

// NewPool creates a worker pool with provided size.
//
// Result is the type of the results returned by the tasks in the pool.
func NewPool[Result any](ctx context.Context, size int) (*Pool[Result], context.Context) {
	pool, ctx := NewSharedPool[Result](ctx, make(PoolSlots, size))
	pool.dedicatedPool = true
	return pool, ctx
}

// NewSharedPool creates a worker pool that shares the provided pool slots.
//
// Result is the type of the results returned by the tasks in the pool.
func NewSharedPool[Result any](ctx context.Context, sharedSlots PoolSlots) (*Pool[Result], context.Context) {
	ctxWithCancel, cancel := context.WithCancelCause(ctx)
	return &Pool[Result]{
		poolSlots: sharedSlots,
		ctx:       ctxWithCancel,
		cancel:    cancel,
		done:      make(chan struct{}),
	}, ctxWithCancel
}

// Go starts a concurrent task in the pool if a slot in the pool is
// available, or blocks until a slot becomes available.
//
// It returns an error if the pool has already been completed or if the context is done.
func (p *Pool[Result]) Go(task func() (Result, error)) error {
	select {
	case <-p.ctx.Done():
		if cause := context.Cause(p.ctx); cause != nil && cause != context.Canceled {
			return cause
		}
		return p.ctx.Err()
	case <-p.done:
		return errors.New("pool has already been completed")
	case p.poolSlots <- slot{}:
		// acquired a slot in the pool
	}

	p.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// capture the panic and cancel the context
				p.panicOnce.Do(func() {
					p.panicValue = r
					p.cancel(errors.New("goroutine panicked"))
				})
			}
			<-p.poolSlots // release pool slot
			p.wg.Done()
		}()

		// execute task
		result, err := task()
		if err != nil {
			p.errOnce.Do(func() {
				p.cancel(err)
			})
		}

		// add result for both success and error cases
		p.resultsMu.Lock()
		p.results = append(p.results, result)
		p.resultsMu.Unlock()
	}()

	return nil
}

// Wait blocks until all tasks in the pool have completed.
func (p *Pool[Result]) Wait() ([]Result, error) {
	if !p.waitOnce() {
		return nil, errors.New("Wait() can only be called once")
	}

	defer func() {
		if p.dedicatedPool {
			close(p.poolSlots)
		}
	}()

	// convert g.wg.Wait() to a channel to avoid blocking
	go func() {
		p.wg.Wait()
		close(p.done)
	}()

	select {
	case <-p.done:
		// all tasks completed normally
	case <-p.ctx.Done():
		// context was cancelled, then wait for cleanup
		<-p.done
	}

	// check if a panic occurred and re-raise it
	if p.panicValue != nil {
		panic(p.panicValue)
	}

	if cause := context.Cause(p.ctx); cause != nil {
		return p.results, cause
	}
	if p.ctx.Err() != nil {
		return p.results, p.ctx.Err()
	}
	return p.results, nil
}

func (p *Pool[Result]) waitOnce() bool {
	return p.hasWaited.CompareAndSwap(false, true)
}
