package link

import (
	"github.com/google/gousb"
	"github.com/mzyy94/gocarplay/protocol"
)

func SendMessage(epOut *gousb.OutEndpoint, msg interface{}) error {
	// Protect USB writes with mutex to prevent concurrent access
	// This ensures touch events and heartbeats don't interfere with each other
	writeMutex.Lock()
	defer writeMutex.Unlock()

	buf, err := protocol.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = epOut.Write(buf[:16])
	if err != nil {
		return err
	}
	if len(buf) > 16 {
		_, err = epOut.Write(buf[16:])
	}
	return err
}
