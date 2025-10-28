package link

import (
	"sync"
)

// ConnectionState represents the current state of the USB dongle connection
type ConnectionState int

const (
	// StateDisconnected indicates no dongle is connected
	StateDisconnected ConnectionState = iota
	// StateConnecting indicates a connection attempt is in progress
	StateConnecting
	// StateConnected indicates the dongle is connected and operational
	StateConnected
)

// String returns the string representation of the connection state
func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	default:
		return "unknown"
	}
}

// StateManager manages the connection state of the USB dongle
type StateManager struct {
	mu        sync.RWMutex
	state     ConnectionState
	listeners []chan ConnectionState
}

// NewStateManager creates a new state manager
func NewStateManager() *StateManager {
	return &StateManager{
		state:     StateDisconnected,
		listeners: make([]chan ConnectionState, 0),
	}
}

// GetState returns the current connection state
func (sm *StateManager) GetState() ConnectionState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

// SetState updates the connection state and notifies listeners
func (sm *StateManager) SetState(newState ConnectionState) {
	sm.mu.Lock()
	oldState := sm.state
	sm.state = newState
	listeners := sm.listeners
	sm.mu.Unlock()

	// Only notify if state actually changed
	if oldState != newState {
		// Notify all listeners (non-blocking)
		for _, ch := range listeners {
			select {
			case ch <- newState:
			default:
				// Skip if channel is full
			}
		}
	}
}

// Subscribe creates a new channel that will receive state change notifications
// The returned channel should be read by the caller
func (sm *StateManager) Subscribe() chan ConnectionState {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	ch := make(chan ConnectionState, 10) // Buffered to prevent blocking
	sm.listeners = append(sm.listeners, ch)
	return ch
}

// Unsubscribe removes a listener channel
func (sm *StateManager) Unsubscribe(ch chan ConnectionState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for i, listener := range sm.listeners {
		if listener == ch {
			sm.listeners = append(sm.listeners[:i], sm.listeners[i+1:]...)
			close(ch)
			break
		}
	}
}

// IsConnected returns true if the dongle is connected
func (sm *StateManager) IsConnected() bool {
	return sm.GetState() == StateConnected
}
