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

package syncutil

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWorkerPool_Basic(t *testing.T) {
	poolSlots := make(PoolSlots, 2)

	ctx := context.Background()
	pool, _ := NewSharedWorkerPool[int](ctx, poolSlots)

	// test empty group
	results, err := pool.Wait()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty results, got %v", results)
	}
}

func TestWorkerPool_SingleTask(t *testing.T) {
	poolSlots := make(PoolSlots, 2)

	ctx := context.Background()
	pool, _ := NewSharedWorkerPool[int](ctx, poolSlots)

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

func TestWorkerPool_MultipleTasks(t *testing.T) {
	poolSlots := make(PoolSlots, 3)
	ctx := context.Background()
	pool, _ := NewSharedWorkerPool[int](ctx, poolSlots)

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

func TestWorkerPool_WithError(t *testing.T) {
	poolSlots := make(PoolSlots, 2)

	ctx := context.Background()
	pool, _ := NewSharedWorkerPool[int](ctx, poolSlots)

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

func TestWorkerPool_WaitCalledTwice(t *testing.T) {
	poolSlots := make(PoolSlots, 2)

	ctx := context.Background()
	pool, _ := NewSharedWorkerPool[int](ctx, poolSlots)

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

func TestWorkerPool_PanicRecovery(t *testing.T) {
	poolSlots := make(PoolSlots, 2)

	ctx := context.Background()
	pool, _ := NewSharedWorkerPool[int](ctx, poolSlots)

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
		if r := recover(); r == nil {
			t.Fatal("expected panic to be re-raised, but no panic occurred")
		}
	}()

	_, _ = pool.Wait()
	t.Fatal("Wait() should have panicked but returned normally")
}

func TestWorkerPool_GoAfterWait(t *testing.T) {
	poolSlots := make(PoolSlots, 2)

	ctx := context.Background()
	pool, _ := NewSharedWorkerPool[int](ctx, poolSlots)

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

func TestWorkerPool_DedicatedPoolClosesSlots(t *testing.T) {
	ctx := context.Background()
	pool, _ := NewWorkerPool[int](ctx, 2) // Create a dedicated pool with size 2

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
