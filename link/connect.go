package link

import (
	"errors"
	"log"
	"time"

	"github.com/google/gousb"
)

// KnownDevices contains the list of known CarPlay dongle USB device IDs
var KnownDevices = []struct {
	VendorID  uint16
	ProductID uint16
}{
	{VendorID: 0x1314, ProductID: 0x1520},
	{VendorID: 0x1314, ProductID: 0x1521},
}

func Connect() (*gousb.InEndpoint, *gousb.OutEndpoint, func(), error) {
	return ConnectWithTimeout(5, 3*time.Second)
}

// ConnectOnce attempts to connect to a CarPlay dongle without retries
// This is useful for hotplug scenarios where the device presence is already confirmed
func ConnectOnce() (*gousb.InEndpoint, *gousb.OutEndpoint, func(), error) {
	return ConnectWithTimeout(0, 0) // No retries, immediate attempt only
}

// ConnectWithTimeout attempts to connect to a CarPlay dongle with configurable retry settings
func ConnectWithTimeout(maxRetries int, retryDelay time.Duration) (*gousb.InEndpoint, *gousb.OutEndpoint, func(), error) {
	cleanTask := make([]func(), 0)
	defer func() {
		for _, task := range cleanTask {
			task()
		}
	}()

	log.Println("[USB] Searching for CarPlay dongle...")
	ctx := gousb.NewContext()
	cleanTask = append(cleanTask, func() { ctx.Close() })

	var (
		dev       *gousb.Device
		err       error
		waitCount = maxRetries
	)

	for {
		// Try each known device
		for _, device := range KnownDevices {
			dev, err = ctx.OpenDeviceWithVIDPID(gousb.ID(device.VendorID), gousb.ID(device.ProductID))
			if err != nil {
				continue // Try next device
			}
			if dev != nil {
				log.Printf("[USB] Found device: VendorID=0x%04x, ProductID=0x%04x", device.VendorID, device.ProductID)
				cleanTask = append(cleanTask, func() { dev.Close() })
				goto deviceFound
			}
		}

		// No device found, retry or fail
		waitCount--
		if waitCount < 0 {
			log.Println("[USB] ERROR: Could not find a compatible CarPlay dongle device")
			return nil, nil, nil, errors.New("Could not find a compatible CarPlay dongle device")
		}
		log.Printf("[USB] Device not found, retrying... (%d attempts remaining)", waitCount)
		time.Sleep(retryDelay)
	}

deviceFound:

	intf, done, err := dev.DefaultInterface()
	if err != nil {
		log.Printf("[USB] ERROR: Failed to claim interface: %v", err)
		return nil, nil, nil, err
	}
	cleanTask = append(cleanTask, done)

	epOut, err := intf.OutEndpoint(1)
	if err != nil {
		log.Printf("[USB] ERROR: Failed to open OUT endpoint: %v", err)
		return nil, nil, nil, err
	}
	epIn, err := intf.InEndpoint(1)
	if err != nil {
		log.Printf("[USB] ERROR: Failed to open IN endpoint: %v", err)
		return nil, nil, nil, err
	}

	log.Println("[USB] Successfully connected to CarPlay dongle")
	log.Printf("[USB] Endpoints configured: IN=0x%02x, OUT=0x%02x", epIn.Desc.Address, epOut.Desc.Address)

	closeTask := make([]func(), len(cleanTask))
	copy(closeTask, cleanTask)
	cleanTask = nil

	return epIn, epOut, func() {
		for _, task := range closeTask {
			task()
		}
	}, nil
}
