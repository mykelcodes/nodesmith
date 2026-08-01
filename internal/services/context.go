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

	uiReady          bool
	uiReadyCallbacks []func(context.Context)
}

func NewBridgeContext() *BridgeContext {
	return &BridgeContext{}
}

// OnUIReady registers a callback to run once the web view can receive events.
// The runtime context exists from application startup, but an event emitted
// before the interface has subscribed is delivered to nobody, so anything the
// interface must observe at launch has to wait for this signal.
func (bridge *BridgeContext) OnUIReady(callback func(context.Context)) {
	if callback == nil {
		return
	}
	bridge.mu.Lock()
	if bridge.uiReady && bridge.context != nil {
		ctx := bridge.context
		bridge.mu.Unlock()
		callback(ctx)
		return
	}
	bridge.uiReadyCallbacks = append(bridge.uiReadyCallbacks, callback)
	bridge.mu.Unlock()
}

// NotifyUIReady is called by the application's DOM-ready hook.
func (bridge *BridgeContext) NotifyUIReady() {
	bridge.mu.Lock()
	bridge.uiReady = true
	ctx := bridge.context
	callbacks := bridge.uiReadyCallbacks
	bridge.uiReadyCallbacks = nil
	bridge.mu.Unlock()

	if ctx == nil {
		// Startup has not supplied a context yet. Keep the callbacks queued
		// rather than dropping the notifications they carry.
		bridge.mu.Lock()
		bridge.uiReadyCallbacks = append(callbacks, bridge.uiReadyCallbacks...)
		bridge.mu.Unlock()
		return
	}
	for _, callback := range callbacks {
		callback(ctx)
	}
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
