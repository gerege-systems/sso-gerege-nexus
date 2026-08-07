package resilience_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/platform/resilience"
)

func TestCircuitBreaker(t *testing.T) {
	cb := resilience.NewCircuitBreaker(1.5, 5*time.Second, 5)

	err := cb.Do(func() error {
		return nil
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestLoadShedder(t *testing.T) {
	ls := resilience.NewLoadShedder(2) // Max 2 concurrent
	handler := ls.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	var wg sync.WaitGroup
	var successCount, shedCount int64

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			switch rec.Code {
			case http.StatusOK:
				atomic.AddInt64(&successCount, 1)
			case http.StatusServiceUnavailable:
				atomic.AddInt64(&shedCount, 1)
			}
		}()
	}

	wg.Wait()
	if successCount == 0 {
		t.Errorf("expected some successful requests")
	}
}

func TestSingleflight(t *testing.T) {
	sf := resilience.NewSingleflight()
	var execCount int32

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := sf.Do("key1", func() (interface{}, error) {
				atomic.AddInt32(&execCount, 1)
				time.Sleep(20 * time.Millisecond)
				return "data", nil
			})
			if err != nil || res != "data" {
				t.Errorf("unexpected singleflight result")
			}
		}()
	}

	wg.Wait()
	if execCount != 1 {
		t.Errorf("expected exactly 1 execution due to singleflight coalescing, got %d", execCount)
	}
}

func TestDoWithRetry(t *testing.T) {
	ctx := context.Background()
	var attempts int

	err := resilience.DoWithRetry(ctx, func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return errors.New("transient error")
		}
		return nil
	}, resilience.RetryOptions{MaxAttempts: 3, InitialWait: 5 * time.Millisecond})

	if err != nil {
		t.Fatalf("expected retry to succeed on 2nd attempt, got error: %v", err)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}
