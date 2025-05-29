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

func TestSimpleGroup_Basic(t *testing.T) {
	pool, err := NewPool(2)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	ctx := context.Background()
	group, _ := NewSimpleGroup[int](ctx, pool)

	// Test empty group
	results, err := group.Wait()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty results, got %v", results)
	}
}

func TestSimpleGroup_SingleTask(t *testing.T) {
	pool, err := NewPool(2)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	ctx := context.Background()
	group, _ := NewSimpleGroup[int](ctx, pool)

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

func TestSimpleGroup_MultipleTasks(t *testing.T) {
	pool, err := NewPool(3)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	ctx := context.Background()
	group, _ := NewSimpleGroup[int](ctx, pool)

	for i := 1; i <= 5; i++ {
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

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
}

func TestSimpleGroup_WithError(t *testing.T) {
	pool, err := NewPool(2)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	ctx := context.Background()
	group, _ := NewSimpleGroup[int](ctx, pool)

	expectedErr := errors.New("task error")
	
	err = group.Submit(func() (int, error) {
		return 0, expectedErr
	})
	if err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	err = group.Submit(func() (int, error) {
		time.Sleep(100 * time.Millisecond) // simulate work
		return 42, nil
	})
	if err != nil {
		t.Fatalf("failed to submit task: %v", err)
	}

	_, err = group.Wait()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	
	if err.Error() != expectedErr.Error() {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
}

func TestSimpleGroup_WaitCalledTwice(t *testing.T) {
	pool, err := NewPool(2)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}

	ctx := context.Background()
	group, _ := NewSimpleGroup[int](ctx, pool)

	// First call should succeed
	_, err = group.Wait()
	if err != nil {
		t.Fatalf("first Wait() call failed: %v", err)
	}

	// Second call should fail
	_, err = group.Wait()
	if err == nil {
		t.Fatal("expected error on second Wait() call, got nil")
	}
}
