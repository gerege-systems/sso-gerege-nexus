/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team & Claude AI
 * Distributed under the Apache 2.0 License.
 *
 * Background housekeeping for abandoned signing ceremonies.
 */

package esign

import (
	"context"
	"log/slog"
	"time"
)

// sweepInterval is how often abandoned ceremonies are closed.
const sweepInterval = 5 * time.Minute

// StartHousekeeping closes signing sessions nobody came back to.
//
// A session holds the document bytes it is signing, so rows left pending
// forever are the module's only unbounded growth. This is a sweep of
// abandonment, not a deadline imposed on a citizen: the horizon is generous
// precisely so it never cuts short somebody who is still unlocking a phone to
// find the notification. eID's own EXPIRED state is what ends a real wait.
//
// It returns when ctx is cancelled, so shutdown does not have to wait a full
// interval.
func (m *Module) StartHousekeeping(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(sweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Bounded so a slow database cannot pile sweeps on top of one
				// another once the interval comes round again.
				sweepCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				closed, err := m.store.expireStaleSessions(sweepCtx)
				cancel()
				if err != nil {
					slog.Warn("esign: could not expire abandoned signing sessions", "error", err)
					continue
				}
				if closed > 0 {
					slog.Info("esign: closed abandoned signing sessions", "count", closed)
				}
			}
		}
	}()
}
