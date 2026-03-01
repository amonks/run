package executor_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/amonks/run/internal/executor"
)

func TestExecuteAndWait(t *testing.T) {
	exec := executor.New()
	expected := errors.New("done")

	exec.Execute(context.Background(), func(ctx context.Context) error {
		return expected
	})

	<-exec.Done()

	if err := exec.Err(); err != expected {
		t.Errorf("expected %v, got %v", expected, err)
	}
}

func TestExecuteNilError(t *testing.T) {
	exec := executor.New()

	exec.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})

	<-exec.Done()

	if err := exec.Err(); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestCancelBlocksUntilExit(t *testing.T) {
	exec := executor.New()
	started := make(chan struct{})

	exec.Execute(context.Background(), func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		// Simulate cleanup work.
		time.Sleep(50 * time.Millisecond)
		return ctx.Err()
	})

	<-started

	err := exec.Cancel()
	// After Cancel returns, Done must be closed.
	select {
	case <-exec.Done():
	default:
		t.Fatal("Done channel not closed after Cancel returned")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestCancelIsIdempotent(t *testing.T) {
	exec := executor.New()

	exec.Execute(context.Background(), func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})

	err1 := exec.Cancel()
	err2 := exec.Cancel()

	if err1 != err2 {
		t.Errorf("expected identical errors, got %v and %v", err1, err2)
	}
}

func TestCancelConcurrent(t *testing.T) {
	exec := executor.New()

	exec.Execute(context.Background(), func(ctx context.Context) error {
		<-ctx.Done()
		time.Sleep(50 * time.Millisecond)
		return ctx.Err()
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			exec.Cancel()
		}()
	}
	wg.Wait()

	select {
	case <-exec.Done():
	default:
		t.Fatal("Done channel not closed after concurrent Cancels")
	}
}

func TestIsIdentity(t *testing.T) {
	a := executor.New()
	b := executor.New()

	if !a.Is(a) {
		t.Error("executor should be equal to itself")
	}
	if a.Is(b) {
		t.Error("different executors should not be equal")
	}
	if b.Is(a) {
		t.Error("different executors should not be equal (reverse)")
	}
}

func TestDoneMultipleListeners(t *testing.T) {
	exec := executor.New()

	exec.Execute(context.Background(), func(ctx context.Context) error {
		return nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-exec.Done():
			case <-time.After(time.Second):
				t.Error("timed out waiting for Done")
			}
		}()
	}
	wg.Wait()
}

func TestParentContextCancellation(t *testing.T) {
	exec := executor.New()
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})

	exec.Execute(ctx, func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})

	<-started
	cancel()
	<-exec.Done()

	if !errors.Is(exec.Err(), context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", exec.Err())
	}
}
