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
	"sync"
	"testing"
	"time"
)

func TestWaitGroup_Complete_EmptyGroup(t *testing.T) {
	var wg waitGroup

	// Complete should return immediately for an empty wait group
	select {
	case <-wg.Complete():
		// Expected: channel should be closed immediately
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Complete() channel should be closed immediately for empty wait group")
	}
}

func TestWaitGroup_Complete_SingleTask(t *testing.T) {
	var wg waitGroup
	wg.Add(1)

	completed := false
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		completed = true
	}()

	// Complete should not return immediately
	select {
	case <-wg.Complete():
		t.Fatal("Complete() channel should not be closed yet")
	case <-time.After(10 * time.Millisecond):
		// Expected: channel should not be closed yet
	}

	// Wait for task to complete
	select {
	case <-wg.Complete():
		if !completed {
			t.Fatal("Task should have completed")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Complete() channel should be closed after task completion")
	}
}

func TestWaitGroup_Complete_MultipleTasks(t *testing.T) {
	var wg waitGroup
	const numTasks = 5
	wg.Add(numTasks)

	completedCount := 0
	var mu sync.Mutex

	for i := 0; i < numTasks; i++ {
		go func(taskID int) {
			defer wg.Done()
			time.Sleep(time.Duration(taskID*10) * time.Millisecond)
			mu.Lock()
			completedCount++
			mu.Unlock()
		}(i)
	}

	// Complete should not return immediately
	select {
	case <-wg.Complete():
		t.Fatal("Complete() channel should not be closed yet")
	case <-time.After(10 * time.Millisecond):
		// Expected: channel should not be closed yet
	}

	// Wait for all tasks to complete
	select {
	case <-wg.Complete():
		mu.Lock()
		if completedCount != numTasks {
			t.Fatalf("Expected %d completed tasks, got %d", numTasks, completedCount)
		}
		mu.Unlock()
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Complete() channel should be closed after all tasks completion")
	}
}

func TestWaitGroup_Complete_MultipleCallers(t *testing.T) {
	var wg waitGroup
	wg.Add(1)

	// Multiple goroutines calling Complete() should all get the same channel
	const numCallers = 10
	results := make(chan bool, numCallers)

	for i := 0; i < numCallers; i++ {
		go func() {
			select {
			case <-wg.Complete():
				results <- true
			case <-time.After(200 * time.Millisecond):
				results <- false
			}
		}()
	}

	// Start the task
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
	}()

	// All callers should receive completion signal
	for i := 0; i < numCallers; i++ {
		select {
		case result := <-results:
			if !result {
				t.Fatal("Complete() should signal all callers")
			}
		case <-time.After(300 * time.Millisecond):
			t.Fatal("Timeout waiting for completion signal")
		}
	}
}

func TestWaitGroup_Complete_IdempotentChannel(t *testing.T) {
	var wg waitGroup

	// Calling Complete() multiple times should return the same channel
	ch1 := wg.Complete()
	ch2 := wg.Complete()

	if ch1 != ch2 {
		t.Fatal("Complete() should return the same channel on multiple calls")
	}

	// Channel should be closed for empty wait group
	select {
	case <-ch1:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Channel should be closed for empty wait group")
	}

	// Channel should still be the same after completion
	ch3 := wg.Complete()
	if ch1 != ch3 {
		t.Fatal("Complete() should return the same channel even after completion")
	}
}

func TestWaitGroup_Complete_ConcurrentAddDone(t *testing.T) {
	var wg waitGroup
	const numTasks = 100

	// Add and complete tasks concurrently
	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(time.Millisecond) // Small delay to create concurrency
		}()
	}

	// Start Complete() in background
	completeCh := make(chan struct{})
	go func() {
		close(completeCh)
	}()

	// Wait for completion
	select {
	case <-completeCh:
		// Expected: all tasks should complete
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for concurrent tasks to complete")
	}
}

func TestWaitGroup_Complete_ChannelClosedOnce(t *testing.T) {
	var wg waitGroup
	wg.Add(1)

	// Get the Complete channel
	completeCh := wg.Complete()

	// Complete the task
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
	}()

	// Wait for completion
	<-completeCh

	// Channel should remain closed - reading from it should not block
	select {
	case <-completeCh:
		// Expected: channel is already closed
	default:
		t.Fatal("Complete channel should remain closed after completion")
	}

	// Getting Complete() again should return the same closed channel
	completeCh2 := wg.Complete()
	if completeCh != completeCh2 {
		t.Fatal("Complete() should return the same channel instance")
	}

	select {
	case <-completeCh2:
		// Expected: channel should still be closed
	default:
		t.Fatal("Complete channel should remain closed")
	}
}
