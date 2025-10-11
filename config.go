package gocarplay

import "github.com/mzyy94/gocarplay/protocol"

// PhoneTypeConfig contains phone-specific configuration
type PhoneTypeConfig struct {
	FrameInterval *int `json:"frameInterval"`
}

// DongleConfig contains all configuration for the CarPlay dongle
type DongleConfig struct {
	AndroidWorkMode        bool                            `json:"androidWorkMode"`
	AutoDetectAndroidMode  bool                            `json:"autoDetectAndroidMode"` // Auto-enable Android mode when Android device connects
	Width                  int32                           `json:"width"`
	Height                 int32                           `json:"height"`
	Fps                    int32                           `json:"fps"`
	Dpi                    int32                           `json:"dpi"`
	Format                 int32                           `json:"format"`
	IBoxVersion            int32                           `json:"iBoxVersion"`
	PacketMax              int32                           `json:"packetMax"`
	PhoneWorkMode          int32                           `json:"phoneWorkMode"`
	NightMode              bool                            `json:"nightMode"`
	BoxName                string                          `json:"boxName"`
	Hand                   protocol.HandDriveType          `json:"hand"`
	MediaDelay             int32                           `json:"mediaDelay"`
	AudioTransferMode      bool                            `json:"audioTransferMode"`
	WifiType               string                          `json:"wifiType"` // "2.4ghz" or "5ghz"
	WifiChannel            int32                           `json:"wifiChannel"`
	MicType                string                          `json:"micType"` // "box" or "os"
	PhoneConfig            map[protocol.PhoneType]*PhoneTypeConfig `json:"phoneConfig"`
}

// DefaultConfig returns the default configuration for the dongle
func DefaultConfig() *DongleConfig {
	frameInterval5000 := 5000
	return &DongleConfig{
		Width:         800,
		Height:        480,
		Fps:           60,
		Dpi:           140,
		Format:        5,
		IBoxVersion:   2,
		PhoneWorkMode: 2,
		PacketMax:     49152,
		BoxName:       "goCarPlay",
		NightMode:     true,
		Hand:          protocol.LHD,
		MediaDelay:    1000,
		AudioTransferMode: false,
		AutoDetectAndroidMode: true, // Enable auto-detection by default
		WifiType:      "5ghz",
		WifiChannel:   36,
		MicType:       "os",
		PhoneConfig: map[protocol.PhoneType]*PhoneTypeConfig{
			protocol.PhoneTypeCarPlay: {FrameInterval: &frameInterval5000},
			protocol.AndroidAuto: {FrameInterval: nil},
		},
	}
}

// GetWifiChannel returns the appropriate WiFi channel based on the WifiType
func (c *DongleConfig) GetWifiChannel() int32 {
	if c.WifiChannel > 0 {
		return c.WifiChannel
	}
	if c.WifiType == "5ghz" {
		return 36
	}
	return 1
}

// GetWifiCommand returns the appropriate WiFi command for the configuration
func (c *DongleConfig) GetWifiCommand() protocol.CarPlayType {
	if c.WifiType == "5ghz" {
		return protocol.Wifi5g
	}
	return protocol.Wifi24g
}

// GetMicCommand returns the appropriate microphone command for the configuration
func (c *DongleConfig) GetMicCommand() protocol.CarPlayType {
	if c.MicType == "box" {
		return protocol.BoxMicrophone
	}
	return protocol.CarMicrophone
}

// GetAudioTransferCommand returns the appropriate audio transfer command
func (c *DongleConfig) GetAudioTransferCommand() protocol.CarPlayType {
	if c.AudioTransferMode {
		return protocol.AudioTransferOn
	}
	return protocol.AudioTransferOff
}