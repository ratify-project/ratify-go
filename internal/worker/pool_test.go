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
	"testing"
	"time"
)

func TestPool_Basic(t *testing.T) {
	poolSlots := make(PoolSlots, 2)

	ctx := context.Background()
	pool, _ := NewSharedPool[int](ctx, poolSlots)

	// test empty group
	results, err := pool.Wait()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty results, got %v", results)
	}
}

func TestPool_SingleTask(t *testing.T) {
	poolSlots := make(PoolSlots, 2)

	ctx := context.Background()
	pool, _ := NewSharedPool[int](ctx, poolSlots)

	err := pool.Go(func() (int, error) {
		return 42, nil
	})
	if err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	results, err := pool.Wait()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(results) != 1 || results[0] != 42 {
		t.Fatalf("expected [42], got %v", results)
	}
}

func TestPool_MultipleTasks(t *testing.T) {
	poolSlots := make(PoolSlots, 3)
	ctx := context.Background()
	pool, _ := NewSharedPool[int](ctx, poolSlots)

	for i := 1; i <= 5; i++ {
		i := i // capture loop variable
		err := pool.Go(func() (int, error) {
			return i, nil
		})
		if err != nil {
			t.Fatalf("failed to submit task %d: %v", i, err)
		}
	}

	results, err := pool.Wait()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
}

func TestPool_WithError(t *testing.T) {
	poolSlots := make(PoolSlots, 2)

	ctx := context.Background()
	pool, _ := NewSharedPool[int](ctx, poolSlots)

	expectedErr := errors.New("task error")

	err := pool.Go(func() (int, error) {
		return 0, expectedErr
	})
	if err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	err = pool.Go(func() (int, error) {
		time.Sleep(100 * time.Millisecond) // simulate work
		return 42, nil
	})
	if err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	_, err = pool.Wait()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if err.Error() != expectedErr.Error() {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
}

func TestPool_WaitCalledTwice(t *testing.T) {
	poolSlots := make(PoolSlots, 2)

	ctx := context.Background()
	pool, _ := NewSharedPool[int](ctx, poolSlots)

	// first call should succeed
	_, err := pool.Wait()
	if err != nil {
		t.Fatalf("first Wait() call failed: %v", err)
	}

	// second call should fail
	_, err = pool.Wait()
	if err == nil {
		t.Fatal("expected error on second Wait() call, got nil")
	}
}

func TestPool_PanicRecovery(t *testing.T) {
	poolSlots := make(PoolSlots, 2)

	ctx := context.Background()
	pool, _ := NewSharedPool[int](ctx, poolSlots)

	// add a task that panics
	err := pool.Go(func() (int, error) {
		panic("test panic")
	})
	if err != nil {
		t.Fatalf("unexpected error adding task: %v", err)
	}

	// add a normal task
	err = pool.Go(func() (int, error) {
		return 42, nil
	})
	if err != nil {
		t.Fatalf("unexpected error adding task: %v", err)
	}

	// wait should re-raise the panic
	defer func() {
		if r := recover(); r != nil {
			if r != "test panic" {
				t.Fatalf("expected panic value 'test panic', got %v", r)
			}
		} else {
			t.Fatal("expected panic to be re-raised, but no panic occurred")
		}
	}()

	_, _ = pool.Wait()
	t.Fatal("Wait() should have panicked but returned normally")
}

func TestPool_MultiplePanics(t *testing.T) {
	poolSlots := make(PoolSlots, 3)

	ctx := context.Background()
	pool, _ := NewSharedPool[int](ctx, poolSlots)

	// add multiple tasks that panic
	err := pool.Go(func() (int, error) {
		panic("first panic")
	})
	if err != nil {
		t.Fatalf("unexpected error adding task: %v", err)
	}

	err = pool.Go(func() (int, error) {
		panic("second panic")
	})
	if err != nil {
		t.Fatalf("unexpected error adding task: %v", err)
	}

	// only the first panic should be captured and re-raised
	defer func() {
		if r := recover(); r != nil {
			// should be one of the panic values, but we can't guarantee which one
			// since goroutines execute concurrently
			if r != "first panic" && r != "second panic" {
				t.Fatalf("expected panic value 'first panic' or 'second panic', got %v", r)
			}
		} else {
			t.Fatal("expected panic to be re-raised, but no panic occurred")
		}
	}()

	_, _ = pool.Wait()
	t.Fatal("Wait() should have panicked but returned normally")
}

func TestPool_GoAfterWait(t *testing.T) {
	poolSlots := make(PoolSlots, 2)

	ctx := context.Background()
	pool, _ := NewSharedPool[int](ctx, poolSlots)

	// Add a task to the pool
	err := pool.Go(func() (int, error) {
		return 42, nil
	})
	if err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	// Wait for all tasks to complete
	results, err := pool.Wait()
	if err != nil {
		t.Fatalf("Wait() failed: %v", err)
	}

	if len(results) != 1 || results[0] != 42 {
		t.Fatalf("expected [42], got %v", results)
	}

	// Try to add another task after Wait() has been called
	err = pool.Go(func() (int, error) {
		return 100, nil
	})
	if err == nil {
		t.Fatal("expected error when calling Go() after Wait(), got nil")
	}

	expectedErrMsg := "pool has already been completed"
	if err.Error() != expectedErrMsg {
		t.Fatalf("expected error message %q, got %q", expectedErrMsg, err.Error())
	}
}

func TestPool_DedicatedPoolClosesSlots(t *testing.T) {
	ctx := context.Background()
	pool, _ := NewPool[int](ctx, 2) // Create a dedicated pool with size 2

	// Add some tasks to the pool
	err := pool.Go(func() (int, error) {
		time.Sleep(10 * time.Millisecond) // simulate some work
		return 1, nil
	})
	if err != nil {
		t.Fatalf("failed to submit first task: %v", err)
	}

	err = pool.Go(func() (int, error) {
		time.Sleep(10 * time.Millisecond) // simulate some work
		return 2, nil
	})
	if err != nil {
		t.Fatalf("failed to submit second task: %v", err)
	}

	// Wait for all tasks to complete
	results, err := pool.Wait()
	if err != nil {
		t.Fatalf("Wait() failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Verify that the poolSlots channel is closed
	// Since it's a dedicated pool, the channel should be closed after Wait()
	select {
	case _, ok := <-pool.poolSlots:
		if ok {
			t.Fatal("expected poolSlots channel to be closed, but it's still open")
		}
		// Channel is closed as expected
	default:
		t.Fatal("expected poolSlots channel to be closed and readable, but it's not")
	}
}

func TestPool_ContextCancelledWithoutCause(t *testing.T) {
	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	pool, _ := NewPool[int](ctx, 2)

	// Add a long-running task
	err := pool.Go(func() (int, error) {
		time.Sleep(100 * time.Millisecond) // simulate work
		return 1, nil
	})
	if err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	// Add another task
	err = pool.Go(func() (int, error) {
		time.Sleep(100 * time.Millisecond) // simulate work
		return 2, nil
	})
	if err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	// Cancel the context after a short delay to let tasks start
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel() // This cancels the context without setting a specific cause
	}()

	// Wait should return the context error (not a specific cause)
	_, err = pool.Wait()
	if err == nil {
		t.Fatal("expected an error due to context cancellation, got nil")
	}

	// The error should be context.Canceled (not a specific cause)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled error, got %v", err)
	}
}
