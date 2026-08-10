// Package async starts supervised background work.
package async

import (
	"log/slog"
	"runtime/debug"
)

// Go starts fn in a goroutine and prevents a panic in background work from
// terminating the process. name must identify the operation in logs.
func Go(name string, fn func()) {
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("background task panicked", "task", name, "panic", recovered, "stack", string(debug.Stack()))
			}
		}()
		fn()
	}()
}
