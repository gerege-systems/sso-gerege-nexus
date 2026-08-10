package resilience_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gerege-systems/sso-gerege-nexus/backend/internal/platform/resilience"
)

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
