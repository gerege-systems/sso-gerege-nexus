package mailer_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/mailer"
)

type mockSyncMailer struct {
	mu    sync.Mutex
	calls []string
}

func (m *mockSyncMailer) SendOTP(ctx context.Context, toEmail, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, toEmail+":"+code)
	return nil
}

func TestAsyncOTPMailer(t *testing.T) {
	mockMailer := &mockSyncMailer{}
	asyncMailer := mailer.NewAsyncOTPMailer(mockMailer, 2, 10, 2)

	ok := asyncMailer.EnqueueOTP("user@example.com", "123456")
	if !ok {
		t.Fatal("expected enqueue to succeed")
	}

	time.Sleep(50 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := asyncMailer.Shutdown(ctx)
	if err != nil {
		t.Fatalf("unexpected error during shutdown: %v", err)
	}

	mockMailer.mu.Lock()
	defer mockMailer.mu.Unlock()
	if len(mockMailer.calls) != 1 {
		t.Fatalf("expected 1 mail call, got %d", len(mockMailer.calls))
	}
	if mockMailer.calls[0] != "user@example.com:123456" {
		t.Errorf("unexpected call format: %s", mockMailer.calls[0])
	}
}

// Shutdown must drain what was already accepted instead of racing the workers
// off the queue, and must be safe to call more than once.
func TestAsyncOTPMailerShutdownDrainsQueue(t *testing.T) {
	mockMailer := &mockSyncMailer{}
	asyncMailer := mailer.NewAsyncOTPMailer(mockMailer, 2, 32, 2)

	const total = 16
	for i := 0; i < total; i++ {
		if !asyncMailer.EnqueueOTP("user@example.com", "code") {
			t.Fatalf("enqueue %d rejected", i)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := asyncMailer.Shutdown(ctx); err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}

	mockMailer.mu.Lock()
	got := len(mockMailer.calls)
	mockMailer.mu.Unlock()
	if got != total {
		t.Fatalf("expected all %d queued mails to be delivered, got %d", total, got)
	}

	// A second Shutdown used to close an already-closed channel and panic.
	if err := asyncMailer.Shutdown(ctx); err != nil {
		t.Fatalf("second shutdown returned error: %v", err)
	}

	// Enqueueing after shutdown must be refused, not panic on a closed channel.
	if asyncMailer.EnqueueOTP("late@example.com", "000000") {
		t.Fatal("expected enqueue after shutdown to be rejected")
	}
}
