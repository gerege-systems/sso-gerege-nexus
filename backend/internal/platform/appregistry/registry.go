package appregistry

import (
	"fmt"
	"sync"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal"
)

type Registry struct {
	mu      sync.RWMutex
	modules map[string]internal.Module
}

var globalRegistry = &Registry{
	modules: make(map[string]internal.Module),
}

// Register adds a compile-time module to the global registry.
func Register(m internal.Module) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.modules[m.ID()] = m
}

// Get returns a module by ID from the global registry.
func Get(id string) (internal.Module, bool) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	m, ok := globalRegistry.modules[id]
	return m, ok
}

// List returns all registered modules.
func List() []internal.Module {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()
	list := make([]internal.Module, 0, len(globalRegistry.modules))
	for _, m := range globalRegistry.modules {
		list = append(list, m)
	}
	return list
}

// VerifyModuleExists checks if a module ID is present in the binary.
func VerifyModuleExists(id string) error {
	if _, ok := Get(id); !ok {
		return fmt.Errorf("module %s is not present in binary registry", id)
	}
	return nil
}
