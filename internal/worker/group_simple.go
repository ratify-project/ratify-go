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
)

// SimpleGroup is a much simpler version that uses sync.WaitGroup
// and eliminates most of the complexity while keeping the same interface
type SimpleGroup[Result any] struct {
	pool     Pool
	ctx      context.Context
	cancel   context.CancelCauseFunc
	
	wg       sync.WaitGroup
	mu       sync.Mutex
	results  []Result
	err      error
	errOnce  sync.Once
	
	waitOnce sync.Once
	closed   bool
}

func NewSimpleGroup[Result any](ctx context.Context, sharedPool Pool) (*SimpleGroup[Result], context.Context) {
	ctxWithCancel, cancel := context.WithCancelCause(ctx)
	return &SimpleGroup[Result]{
		pool:   sharedPool,
		ctx:    ctxWithCancel,
		cancel: cancel,
	}, ctxWithCancel
}

func (g *SimpleGroup[Result]) Submit(task func() (Result, error)) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	if g.closed {
		return errors.New("cannot submit tasks after Wait() has been called")
	}
	
	select {
	case <-g.ctx.Done():
		if cause := context.Cause(g.ctx); cause != nil && cause != context.Canceled {
			return cause
		}
		return g.ctx.Err()
	default:
	}
	
	g.wg.Add(1)
	
	go func() {
		defer g.wg.Done()
		
		// Acquire from pool
		select {
		case g.pool <- token{}:
			defer func() { <-g.pool }()
		case <-g.ctx.Done():
			return
		}
		
		// Execute task
		result, err := task()
		if err != nil {
			g.errOnce.Do(func() {
				g.cancel(err)
			})
			return
		}
		
		// Store result
		g.mu.Lock()
		g.results = append(g.results, result)
		g.mu.Unlock()
	}()
	
	return nil
}

func (g *SimpleGroup[Result]) Wait() ([]Result, error) {
	var results []Result
	var err error
	
	g.waitOnce.Do(func() {
		g.mu.Lock()
		g.closed = true
		g.mu.Unlock()
		
		// Wait for all tasks to complete
		done := make(chan struct{})
		go func() {
			g.wg.Wait()
			close(done)
		}()
		
		select {
		case <-done:
			// All tasks completed normally
		case <-g.ctx.Done():
			// Context was cancelled, wait for cleanup
			<-done
		}
		
		g.mu.Lock()
		results = make([]Result, len(g.results))
		copy(results, g.results)
		g.mu.Unlock()
		
		// Check for errors
		if cause := context.Cause(g.ctx); cause != nil && cause != context.Canceled {
			err = cause
			return
		}
		
		if g.ctx.Err() != nil {
			err = g.ctx.Err()
			return
		}
	})
	
	if results == nil && err == nil {
		return nil, errors.New("Wait() can only be called once on Group")
	}
	
	return results, err
}
