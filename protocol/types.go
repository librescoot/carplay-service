package protocol

import (
	"fmt"
	"strings"
)

/// enum type def

type CarPlayType uint32

const (
	Invalid              = CarPlayType(0)
	StartRecordAudio     = CarPlayType(1)
	StopRecordAudio      = CarPlayType(2)
	RequestHostUI        = CarPlayType(3)
	BtnSiri              = CarPlayType(5)
	CarMicrophone        = CarPlayType(7)
	Frame                = CarPlayType(12)
	BoxMicrophone        = CarPlayType(15)
	EnableNightMode      = CarPlayType(16)
	DisableNightMode     = CarPlayType(17)
	AudioTransferOn      = CarPlayType(22)
	AudioTransferOff     = CarPlayType(23)
	Wifi24g              = CarPlayType(24)
	Wifi5g               = CarPlayType(25)
	BtnLeft              = CarPlayType(100)
	BtnRight             = CarPlayType(101)
	BtnSelectDown        = CarPlayType(104)
	BtnSelectUp          = CarPlayType(105)
	BtnBack              = CarPlayType(106)
	BtnUp                = CarPlayType(113)
	BtnDown              = CarPlayType(114)
	BtnHome              = CarPlayType(200)
	BtnPlay              = CarPlayType(201)
	BtnPause             = CarPlayType(202)
	BtnPlayOrPause       = CarPlayType(203)
	BtnNextTrack         = CarPlayType(204)
	BtnPrevTrack         = CarPlayType(205)
	AcceptPhoneCall      = CarPlayType(300)
	RejectPhoneCall      = CarPlayType(301)
	RequestVideoFocus    = CarPlayType(500)
	ReleaseVideoFocus    = CarPlayType(501)
	SupportWifi          = CarPlayType(1000)
	AutoConnectEnable    = CarPlayType(1001)
	WifiConnect          = CarPlayType(1002)
	ScanningDevice       = CarPlayType(1003)
	DeviceFound          = CarPlayType(1004)
	DeviceNotFound       = CarPlayType(1005)
	ConnectDeviceFailed  = CarPlayType(1006)
	BtConnected          = CarPlayType(1007)
	BtDisconnected       = CarPlayType(1008)
	WifiConnected        = CarPlayType(1009)
	WifiDisconnected     = CarPlayType(1010)
	BtPairStart          = CarPlayType(1011)
	SupportWifiNeedKo    = CarPlayType(1012)
)

func (c CarPlayType) GoString() string {
	switch c {
	case 0:
		return "Invalid"
	case 1:
		return "StartRecordAudio"
	case 2:
		return "StopRecordAudio"
	case 3:
		return "RequestHostUI"
	case 5:
		return "BtnSiri"
	case 7:
		return "CarMicrophone"
	case 12:
		return "Frame"
	case 15:
		return "BoxMicrophone"
	case 16:
		return "EnableNightMode"
	case 17:
		return "DisableNightMode"
	case 22:
		return "AudioTransferOn"
	case 23:
		return "AudioTransferOff"
	case 24:
		return "Wifi24g"
	case 25:
		return "Wifi5g"
	case 100:
		return "BtnLeft"
	case 101:
		return "BtnRight"
	case 104:
		return "BtnSelectDown"
	case 105:
		return "BtnSelectUp"
	case 106:
		return "BtnBack"
	case 113:
		return "BtnUp"
	case 114:
		return "BtnDown"
	case 200:
		return "BtnHome"
	case 201:
		return "BtnPlay"
	case 202:
		return "BtnPause"
	case 203:
		return "BtnPlayOrPause"
	case 204:
		return "BtnNextTrack"
	case 205:
		return "BtnPrevTrack"
	case 300:
		return "AcceptPhoneCall"
	case 301:
		return "RejectPhoneCall"
	case 500:
		return "RequestVideoFocus"
	case 501:
		return "ReleaseVideoFocus"
	case 1000:
		return "SupportWifi"
	case 1001:
		return "AutoConnectEnable"
	case 1002:
		return "WifiConnect"
	case 1003:
		return "ScanningDevice"
	case 1004:
		return "DeviceFound"
	case 1005:
		return "DeviceNotFound"
	case 1006:
		return "ConnectDeviceFailed"
	case 1007:
		return "BtConnected"
	case 1008:
		return "BtDisconnected"
	case 1009:
		return "WifiConnected"
	case 1010:
		return "WifiDisconnected"
	case 1011:
		return "BtPairStart"
	case 1012:
		return "SupportWifiNeedKo"
	}
	return fmt.Sprintf("Unknown(%d)", c)
}

type AudioCommand uint8

const (
	AudioOutputStart    = AudioCommand(0x01)
	AudioOutputStop     = AudioCommand(0x02)
	AudioInputConfig    = AudioCommand(0x03)
	AudioPhonecallStart = AudioCommand(0x04)
	AudioPhonecallStop  = AudioCommand(0x05)
	AudioNaviStart      = AudioCommand(0x06)
	AudioNaviStop       = AudioCommand(0x07)
	AudioSiriStart      = AudioCommand(0x08)
	AudioSiriStop       = AudioCommand(0x09)
	AudioMediaStart     = AudioCommand(0x0a)
	AudioMediaStop      = AudioCommand(0x0b)
	AudioAlertStart     = AudioCommand(0x0c)
	AudioAlertStop      = AudioCommand(0x0d)
)

func (c AudioCommand) GoString() string {
	switch c {
	case 0x01:
		return "AudioOutputStart"
	case 0x02:
		return "AudioOutputStop"
	case 0x03:
		return "AudioInputConfig"
	case 0x04:
		return "AudioPhonecallStart"
	case 0x05:
		return "AudioPhonecallStop"
	case 0x06:
		return "AudioNaviStart"
	case 0x07:
		return "AudioNaviStop"
	case 0x08:
		return "AudioSiriStart"
	case 0x09:
		return "AudioSiriStop"
	case 0x0a:
		return "AudioMediaStart"
	case 0x0b:
		return "AudioMediaStop"
	case 0x0c:
		return "AudioAlertStart"
	case 0x0d:
		return "AudioAlertStop"
	}
	return fmt.Sprintf("Unknown(%d)", c)
}

type AudioFormat struct {
	Frequency, Channel, Bitrate uint16
}

type DecodeType uint32

var AudioDecodeTypes = map[DecodeType]AudioFormat{
	0: {0, 0, 0},
	1: {44100, 2, 16},
	2: {48000, 2, 16},
	3: {8000, 1, 16},
	4: {48000, 2, 16},
	5: {16000, 1, 16},
	6: {24000, 1, 16},
	7: {16000, 2, 16},
}

type TouchAction uint32

const (
	TouchDown = TouchAction(14)
	TouchMove = TouchAction(15)
	TouchUp   = TouchAction(16)
)

type NullTermString string

func (s NullTermString) GoString() string {
	return "'" + strings.TrimRight(string(s), "\x00") + "'"
}

type PhoneType uint32

const (
	AndroidMirror   = PhoneType(1)
	PhoneTypeCarPlay = PhoneType(3)
	IPhoneMirror    = PhoneType(4)
	AndroidAuto     = PhoneType(5)
	HiCar           = PhoneType(6)
)

func (p PhoneType) String() string {
	switch p {
	case AndroidMirror:
		return "AndroidMirror"
	case PhoneTypeCarPlay:
		return "CarPlay"
	case IPhoneMirror:
		return "iPhoneMirror"
	case AndroidAuto:
		return "AndroidAuto"
	case HiCar:
		return "HiCar"
	default:
		return fmt.Sprintf("Unknown(%d)", p)
	}
}

type MediaType uint32

const (
	MediaTypeData       = MediaType(1)
	MediaTypeAlbumCover = MediaType(3)
)

type MultiTouchAction uint32

const (
	MultiTouchUp   = MultiTouchAction(0)
	MultiTouchDown = MultiTouchAction(1)
	MultiTouchMove = MultiTouchAction(2)
)

type LogoType uint32

const (
	LogoHomeButton = LogoType(1)
	LogoSiri       = LogoType(2)
)

type HandDriveType uint32

const (
	LHD = HandDriveType(0) // Left-hand drive
	RHD = HandDriveType(1) // Right-hand drive
)

type FileAddress string

const (
	FileAddressDPI            = FileAddress("/tmp/screen_dpi")
	FileAddressNightMode      = FileAddress("/tmp/night_mode")
	FileAddressHandDriveMode  = FileAddress("/tmp/hand_drive_mode")
	FileAddressChargeMode     = FileAddress("/tmp/charge_mode")
	FileAddressBoxName        = FileAddress("/etc/box_name")
	FileAddressOEMIcon        = FileAddress("/etc/oem_icon.png")
	FileAddressAirplayConfig  = FileAddress("/etc/airplay.conf")
	FileAddressIcon120        = FileAddress("/etc/icon_120x120.png")
	FileAddressIcon180        = FileAddress("/etc/icon_180x180.png")
	FileAddressIcon256        = FileAddress("/etc/icon_256x256.png")
	FileAddressAndroidWorkMode = FileAddress("/etc/android_work_mode")
)
