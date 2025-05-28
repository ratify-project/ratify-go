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
	"testing"
	"time"
)

func TestGroup_EmptyGroup(t *testing.T) {
	pool, err := NewPool(2)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	ctx := context.Background()
	group, _ := NewGroup[int](ctx, pool)

	results, err := group.Wait()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("expected empty results, got %v", results)
	}
}

func TestGroup_SingleTask(t *testing.T) {
	pool, err := NewPool(2)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	ctx := context.Background()
	group, _ := NewGroup[int](ctx, pool)

	err = group.Submit(func() (int, error) {
		return 42, nil
	})
	if err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	results, err := group.Wait()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(results) != 1 || results[0] != 42 {
		t.Fatalf("expected [42], got %v", results)
	}
}

func TestGroup_MultipleTasksSuccess(t *testing.T) {
	pool, err := NewPool(3)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	ctx := context.Background()
	group, _ := NewGroup[int](ctx, pool)

	expectedSum := 0
	for i := 1; i <= 10; i++ {
		expectedSum += i
		i := i // capture loop variable
		err = group.Submit(func() (int, error) {
			return i, nil
		})
		if err != nil {
			t.Fatalf("failed to submit task %d: %v", i, err)
		}
	}

	results, err := group.Wait()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(results) != 10 {
		t.Fatalf("expected 10 results, got %d", len(results))
	}

	sum := 0
	for _, result := range results {
		sum += result
	}

	if sum != expectedSum {
		t.Fatalf("expected sum %d, got %d", expectedSum, sum)
	}
}

func TestGroup_TaskError(t *testing.T) {
	pool, err := NewPool(2)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	ctx := context.Background()
	group, _ := NewGroup[int](ctx, pool)

	expectedError := errors.New("task error")
	err = group.Submit(func() (int, error) {
		return 0, expectedError
	})
	if err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	// Submit another task that should succeed
	err = group.Submit(func() (int, error) {
		time.Sleep(100 * time.Millisecond) // Give first task time to fail
		return 42, nil
	})
	if err != nil {
		t.Fatalf("failed to submit second task: %v", err)
	}

	results, err := group.Wait()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, expectedError) {
		t.Fatalf("expected error %v, got %v", expectedError, err)
	}

	// Results might be nil or partial depending on timing
	_ = results
}

func TestGroup_ContextCancellation(t *testing.T) {
	pool, err := NewPool(2)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	group, _ := NewGroup[int](ctx, pool)

	var wg sync.WaitGroup
	wg.Add(1)

	err = group.Submit(func() (int, error) {
		wg.Wait() // Wait for cancellation
		return 42, nil
	})
	if err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	// Cancel context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
		wg.Done()
	}()

	results, err := group.Wait()
	if err == nil {
		t.Fatalf("expected context cancellation error, got nil")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	_ = results
}

func TestGroup_ConcurrentSubmitAndWait(t *testing.T) {
	pool, err := NewPool(5)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	ctx := context.Background()
	group, _ := NewGroup[int](ctx, pool)

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Submit tasks
	go func() {
		defer wg.Done()
		for i := 1; i <= 100; i++ {
			i := i
			err := group.Submit(func() (int, error) {
				return i, nil
			})
			if err != nil {
				t.Errorf("failed to submit task %d: %v", i, err)
				return
			}
		}
	}()

	// Goroutine 2: Wait for results
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond) // Give submit goroutine a head start
		results, err := group.Wait()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}

		if len(results) != 100 {
			t.Errorf("expected 100 results, got %d", len(results))
		}
	}()

	wg.Wait()
}

func TestGroup_SubmitAfterError(t *testing.T) {
	pool, err := NewPool(2)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	ctx := context.Background()
	group, _ := NewGroup[int](ctx, pool)

	expectedError := errors.New("task error")

	// Submit a task that will fail
	err = group.Submit(func() (int, error) {
		return 0, expectedError
	})
	if err != nil {
		t.Fatalf("failed to submit first task: %v", err)
	}

	// Wait a bit for the first task to fail and cancel the context
	time.Sleep(50 * time.Millisecond)

	// Try to submit another task after the context has been cancelled due to error
	err = group.Submit(func() (int, error) {
		return 42, nil
	})

	// This should return the original task error, not just context.Canceled
	if err == nil {
		t.Fatalf("expected error when submitting after context cancellation")
	}

	if !errors.Is(err, expectedError) {
		t.Fatalf("expected original task error %v, got %v", expectedError, err)
	}
}

func TestGroup_SubmitWithContextCause(t *testing.T) {
	pool, err := NewPool(2)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	originalCause := errors.New("original cause")
	ctx, cancel := context.WithCancelCause(context.Background())
	group, groupCtx := NewGroup[int](ctx, pool)

	// Cancel the context with a specific cause
	cancel(originalCause)

	// Wait a bit for the cancellation to propagate
	time.Sleep(10 * time.Millisecond)

	// Try to submit a task after context cancellation with cause
	err = group.Submit(func() (int, error) {
		return 42, nil
	})

	if err == nil {
		t.Fatalf("expected error when submitting after context cancellation")
	}

	// Should return the original cause, not just context.Canceled
	if !errors.Is(err, originalCause) {
		t.Fatalf("expected original cause %v, got %v", originalCause, err)
	}

	// Verify the group context is also cancelled
	select {
	case <-groupCtx.Done():
		// Expected
	default:
		t.Fatalf("expected group context to be cancelled")
	}
}
