package link

import (
	"log"
	"sync"
	"time"

	"github.com/google/gousb"
)

// HotplugManager handles USB device hotplug events
type HotplugManager struct {
	ctx          *gousb.Context
	stateManager *StateManager
	mu           sync.Mutex
	stopChan     chan struct{}
	doneChan     chan struct{}

	// Connection management
	onConnect    func() error
	onDisconnect func()
}

// NewHotplugManager creates a new hotplug manager
func NewHotplugManager(stateManager *StateManager) *HotplugManager {
	return &HotplugManager{
		stateManager: stateManager,
		stopChan:     make(chan struct{}),
		doneChan:     make(chan struct{}),
	}
}

// SetConnectionCallbacks sets the callbacks for connection events
func (hm *HotplugManager) SetConnectionCallbacks(onConnect func() error, onDisconnect func()) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.onConnect = onConnect
	hm.onDisconnect = onDisconnect
}

// Start begins monitoring for USB hotplug events
func (hm *HotplugManager) Start() error {
	hm.mu.Lock()
	if hm.ctx != nil {
		hm.mu.Unlock()
		return nil // Already started
	}

	ctx := gousb.NewContext()
	hm.ctx = ctx
	hm.mu.Unlock()

	// Start the monitoring goroutine
	go hm.monitorDevices()

	log.Println("USB hotplug monitoring started")
	return nil
}

// Stop stops the hotplug monitoring
func (hm *HotplugManager) Stop() {
	hm.mu.Lock()
	if hm.ctx == nil {
		hm.mu.Unlock()
		return // Not started
	}
	hm.mu.Unlock()

	close(hm.stopChan)
	<-hm.doneChan // Wait for monitoring to stop

	hm.mu.Lock()
	if hm.ctx != nil {
		hm.ctx.Close()
		hm.ctx = nil
	}
	hm.mu.Unlock()

	log.Println("USB hotplug monitoring stopped")
}

// monitorDevices polls for device changes
// Note: libusb hotplug callbacks in gousb have platform limitations,
// so we use a polling approach for reliability across platforms
func (hm *HotplugManager) monitorDevices() {
	defer close(hm.doneChan)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	wasConnected := false

	for {
		select {
		case <-hm.stopChan:
			return
		case <-ticker.C:
			hm.mu.Lock()
			ctx := hm.ctx
			hm.mu.Unlock()

			if ctx == nil {
				return
			}

			// Check if any known device is present
			isConnected := hm.isDevicePresent(ctx)

			// Detect state transitions
			if isConnected && !wasConnected {
				// Device attached
				log.Println("USB dongle detected (hotplug event)")
				hm.handleAttach()
			} else if !isConnected && wasConnected {
				// Device detached
				log.Println("USB dongle removed (hotplug event)")
				hm.handleDetach()
			}

			wasConnected = isConnected
		}
	}
}

// isDevicePresent checks if any known CarPlay device is present
func (hm *HotplugManager) isDevicePresent(ctx *gousb.Context) bool {
	for _, device := range KnownDevices {
		dev, err := ctx.OpenDeviceWithVIDPID(gousb.ID(device.VendorID), gousb.ID(device.ProductID))
		if err == nil && dev != nil {
			dev.Close()
			return true
		}
	}
	return false
}

// handleAttach handles device attachment events
func (hm *HotplugManager) handleAttach() {
	currentState := hm.stateManager.GetState()

	// Only attempt connection if we're currently disconnected
	if currentState != StateDisconnected {
		return
	}

	hm.stateManager.SetState(StateConnecting)

	hm.mu.Lock()
	onConnect := hm.onConnect
	hm.mu.Unlock()

	if onConnect != nil {
		// Run connection attempt in a goroutine to avoid blocking the monitor
		go func() {
			if err := onConnect(); err != nil {
				log.Printf("Failed to connect to newly attached dongle: %v", err)
				hm.stateManager.SetState(StateDisconnected)
			} else {
				log.Println("Successfully connected to newly attached dongle")
				hm.stateManager.SetState(StateConnected)
			}
		}()
	}
}

// handleDetach handles device detachment events
func (hm *HotplugManager) handleDetach() {
	currentState := hm.stateManager.GetState()

	// Only handle detach if we're currently connected or connecting
	if currentState == StateDisconnected {
		return
	}

	hm.mu.Lock()
	onDisconnect := hm.onDisconnect
	hm.mu.Unlock()

	if onDisconnect != nil {
		// Run cleanup in a goroutine to avoid blocking the monitor
		go func() {
			log.Println("Cleaning up after dongle detachment")
			onDisconnect()
			hm.stateManager.SetState(StateDisconnected)
		}()
	} else {
		hm.stateManager.SetState(StateDisconnected)
	}
}

// TriggerConnectionAttempt manually triggers a connection attempt
// Useful for initial connection attempt at startup
func (hm *HotplugManager) TriggerConnectionAttempt() {
	hm.mu.Lock()
	ctx := hm.ctx
	hm.mu.Unlock()

	if ctx != nil && hm.isDevicePresent(ctx) {
		hm.handleAttach()
	}
}
