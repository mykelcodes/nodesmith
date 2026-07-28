package services

import (
	"context"
	"sync"
)

// BridgeContext shares the Wails v2 runtime context with bound service
// adapters without coupling any domain package to Wails.
type BridgeContext struct {
	mu      sync.RWMutex
	context context.Context
}

func NewBridgeContext() *BridgeContext {
	return &BridgeContext{}
}

// Set is called by the application startup hook.
func (bridge *BridgeContext) Set(ctx context.Context) {
	bridge.mu.Lock()
	bridge.context = ctx
	bridge.mu.Unlock()
}

func (bridge *BridgeContext) get() context.Context {
	bridge.mu.RLock()
	ctx := bridge.context
	bridge.mu.RUnlock()
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func (bridge *BridgeContext) ready() (context.Context, bool) {
	bridge.mu.RLock()
	ctx := bridge.context
	bridge.mu.RUnlock()
	return ctx, ctx != nil
}
