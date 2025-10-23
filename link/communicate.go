package link

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/google/gousb"
	"github.com/mzyy94/gocarplay"
	"github.com/mzyy94/gocarplay/protocol"
)

var epIn *gousb.InEndpoint
var epOut *gousb.OutEndpoint
var ctx context.Context
var Done func()
var heartbeatTicker *time.Ticker
var heartbeatDone chan bool
var currentConfig *gocarplay.DongleConfig
var writeMutex sync.Mutex // Protects USB writes from concurrent access

func Init() error {
	var err error
	epIn, epOut, Done, err = Connect()
	if err != nil {
		return err
	}
	ctx = context.Background()
	log.Println("[Link] Initialization complete, ready to communicate")
	return nil
}

func intToByte(data int32) []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, data)
	return buf.Bytes()
}

func boolToByte(data bool) []byte {
	if data {
		return intToByte(1)
	}
	return intToByte(0)
}

// StartWithConfig initializes the dongle with the given configuration
func StartWithConfig(config *gocarplay.DongleConfig) error {
	if config == nil {
		config = gocarplay.DefaultConfig()
	}

	// Store config globally for phone detection
	currentConfig = config

	log.Printf("[Config] Starting with display: %dx%d @ %d fps, DPI: %d", config.Width, config.Height, config.Fps, config.Dpi)

	// Send initial configuration files
	err := SendData(&protocol.SendFile{
		FileName: protocol.NullTermString(protocol.FileAddressDPI + "\x00"),
		Content:  intToByte(config.Dpi),
	})
	if err != nil {
		return err
	}

	// Send Open message
	err = SendData(&protocol.Open{
		Width:          config.Width,
		Height:         config.Height,
		VideoFrameRate: config.Fps,
		Format:         config.Format,
		PacketMax:      config.PacketMax,
		IBoxVersion:    config.IBoxVersion,
		PhoneWorkMode:  config.PhoneWorkMode,
	})
	if err != nil {
		return err
	}
	log.Println("[Config] Open message sent to dongle")

	// Send configuration settings
	SendData(&protocol.SendFile{
		FileName: protocol.NullTermString(protocol.FileAddressNightMode + "\x00"),
		Content:  boolToByte(config.NightMode),
	})

	SendData(&protocol.SendFile{
		FileName: protocol.NullTermString(protocol.FileAddressHandDriveMode + "\x00"),
		Content:  intToByte(int32(config.Hand)),
	})

	SendData(&protocol.SendFile{
		FileName: protocol.NullTermString(protocol.FileAddressChargeMode + "\x00"),
		Content:  boolToByte(true),
	})

	SendData(&protocol.SendFile{
		FileName: protocol.NullTermString(protocol.FileAddressBoxName + "\x00"),
		Content:  []byte(config.BoxName),
	})

	// Send WiFi configuration
	SendData(&protocol.CarPlay{Type: config.GetWifiCommand()})

	// Send Box Settings
	SendBoxSettings(config)

	// Enable WiFi
	SendData(&protocol.CarPlay{Type: protocol.SupportWifi})

	// Configure microphone
	SendData(&protocol.CarPlay{Type: config.GetMicCommand()})

	// Configure audio transfer
	audioCmd := config.GetAudioTransferCommand()
	log.Printf("[Config] Setting audio transfer: %v", audioCmd)
	SendData(&protocol.CarPlay{Type: audioCmd})

	// Send Android work mode if configured
	if config.AndroidWorkMode {
		SendData(&protocol.SendFile{
			FileName: protocol.NullTermString(protocol.FileAddressAndroidWorkMode + "\x00"),
			Content:  boolToByte(config.AndroidWorkMode),
		})
	}

	// Delay before sending WiFi connect
	time.Sleep(600 * time.Millisecond)
	SendData(&protocol.CarPlay{Type: protocol.WifiConnect})

	// Start heartbeat
	startHeartbeat()

	log.Println("[Config] Configuration complete, dongle is ready")
	return nil
}

// SendBoxSettings sends the BoxSettings configuration message
func SendBoxSettings(config *gocarplay.DongleConfig) error {
	settings := map[string]interface{}{
		"mediaDelay":       config.MediaDelay,
		"syncTime":         time.Now().UnixMilli(),
		"androidAutoSizeW": config.Width,
		"androidAutoSizeH": config.Height,
		"WiFiChannel":      config.GetWifiChannel(),
		"wifiChannel":      config.GetWifiChannel(),
	}

	jsonData, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	return SendData(&protocol.BoxSettings{Settings: jsonData})
}

// Start initializes with default width, height, fps, dpi (backward compatibility)
func Start(width, height, fps, dpi int32) {
	config := gocarplay.DefaultConfig()
	config.Width = width
	config.Height = height
	config.Fps = fps
	config.Dpi = dpi
	StartWithConfig(config)
}

func startHeartbeat() {
	if heartbeatTicker != nil {
		return
	}
	heartbeatTicker = time.NewTicker(2 * time.Second)
	heartbeatDone = make(chan bool)

	go func() {
		for {
			select {
			case <-heartbeatDone:
				return
			case <-heartbeatTicker.C:
				SendData(&protocol.Heartbeat{})
			}
		}
	}()
}

func stopHeartbeat() {
	if heartbeatTicker != nil {
		heartbeatTicker.Stop()
		heartbeatDone <- true
		heartbeatTicker = nil
	}
}

func Communicate(onData func(interface{}), onError func(error)) error {
	if epIn == nil {
		return errors.New("Not connected")
	}
	for {
		received, err := ReceiveMessage(epIn, ctx)
		if err != nil {
			onError(err)
		} else {
			onData(received)
		}
	}
}

func SendData(data interface{}) error {
	if epOut == nil {
		return errors.New("Not connected")
	}
	return SendMessage(epOut, data)
}

// Close properly closes the connection and stops the heartbeat
func Close() {
	stopHeartbeat()
	if Done != nil {
		Done()
		Done = nil
	}
	epIn = nil
	epOut = nil
}

// SendCommand sends a CarPlay command
func SendCommand(command protocol.CarPlayType) error {
	return SendData(&protocol.CarPlay{Type: command})
}

// DisconnectPhone sends a disconnect phone message
func DisconnectPhone() error {
	return SendData(&protocol.DisconnectPhone{})
}

// CloseDongle sends a close dongle message
func CloseDongle() error {
	return SendData(&protocol.CloseDongle{})
}

// HandlePhonePlugged processes phone connection events and optionally enables Android work mode
func HandlePhonePlugged(plugged *protocol.Plugged) {
	if currentConfig == nil {
		return
	}

	// Check if auto-detection is enabled
	if !currentConfig.AutoDetectAndroidMode {
		return
	}

	// Check if Android work mode is already enabled
	if currentConfig.AndroidWorkMode {
		return
	}

	// Detect Android devices
	switch plugged.PhoneType {
	case protocol.AndroidAuto, protocol.AndroidMirror:
		// Android device detected - enable Android work mode
		err := SendData(&protocol.SendFile{
			FileName: protocol.NullTermString(protocol.FileAddressAndroidWorkMode + "\x00"),
			Content:  boolToByte(true),
		})
		if err == nil {
			currentConfig.AndroidWorkMode = true
			// Log the auto-enable action (you can pass this to a callback if needed)
			println("Auto-enabled Android work mode for", plugged.PhoneType.String())
		}
	}
}

// IsAndroidDevice checks if the phone type is an Android device
func IsAndroidDevice(phoneType protocol.PhoneType) bool {
	return phoneType == protocol.AndroidAuto || phoneType == protocol.AndroidMirror
}
