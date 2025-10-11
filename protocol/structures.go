package protocol

type SendFile struct {
	FileNameSize int32 `struc:"int32,little,sizeof=FileName"`
	FileName     NullTermString
	ContentSize  int32 `struc:"int32,little,sizeof=Content"`
	Content      []byte
}

type Open struct {
	Width          int32 `struc:"int32,little"`
	Height         int32 `struc:"int32,little"`
	VideoFrameRate int32 `struc:"int32,little"`
	Format         int32 `struc:"int32,little"`
	PacketMax      int32 `struc:"int32,little"`
	IBoxVersion    int32 `struc:"int32,little"`
	PhoneWorkMode  int32 `struc:"int32,little"`
}

type Heartbeat struct {
}

type ManufacturerInfo struct {
	A int32 `struc:"int32,little"`
	B int32 `struc:"int32,little"`
}

type CarPlay struct {
	Type CarPlayType `struc:"int32,little"`
}

type SoftwareVersion struct {
	Version NullTermString `struc:"[32]byte"`
}

type BluetoothAddress struct {
	Address NullTermString `struc:"[17]byte"`
}

type BluetoothPIN struct {
	Address NullTermString `struc:"[4]byte"`
}

type Plugged struct {
	PhoneType PhoneType `struc:"int32,little"`
	Wifi      int32     `struc:"int32,little,omitempty"`
}

type Unplugged struct {
}

type VideoData struct {
	Width    int32  `struc:"int32,little"`
	Height   int32  `struc:"int32,little"`
	Flags    int32  `struc:"int32,little"`
	Length   int32  `struc:"int32,little,sizeof=Data"`
	Unknown2 int32  `struc:"int32,little"`
	Data     []byte `struc:"[]byte"`
}

type AudioData struct {
	DecodeType     DecodeType   `struc:"int32,little"`
	Volume         float32      `struc:"float32,little"`
	AudioType      int32        `struc:"int32,little"`
	Command        AudioCommand `struc:"skip"`
	VolumeDuration int32        `struc:"skip"`
	Data           []byte       `struc:"skip"`
}

type Touch struct {
	Action TouchAction `struc:"int32,little"`
	X      uint32      `struc:"uint32,little"`
	Y      uint32      `struc:"uint32,little"`
	Flags  uint32      `struc:"int32,little"`
}

type MultiTouch struct {
	Touches []TouchItem `struc:"skip"`
}

type TouchItem struct {
	X      float32             `struc:"float32,little"`
	Y      float32             `struc:"float32,little"`
	Action MultiTouchAction    `struc:"int32,little"`
	ID     uint32              `struc:"uint32,little"`
}

type BluetoothDeviceName struct {
	Data NullTermString `struc:"skip"`
}

type WifiDeviceName struct {
	Data NullTermString `struc:"skip"`
}

type BluetoothPairedList struct {
	Data NullTermString `struc:"skip"`
}

type Unknown struct {
	Type uint32 `struc:"skip"`
	Data []byte `struc:"skip"`
}

type MediaData struct {
	Type      MediaType `struc:"int32,little"`
	MediaInfo []byte    `struc:"skip"`
}

type Opened struct {
	Width         int32 `struc:"int32,little"`
	Height        int32 `struc:"int32,little"`
	Fps           int32 `struc:"int32,little"`
	Format        int32 `struc:"int32,little"`
	PacketMax     int32 `struc:"int32,little"`
	IBox          int32 `struc:"int32,little"`
	PhoneMode     int32 `struc:"int32,little"`
}

type BoxSettings struct {
	Settings []byte `struc:"skip"`
}

type Phase struct {
	PhaseValue uint32 `struc:"int32,little"`
}

type HiCarLink struct {
	Link NullTermString `struc:"skip"`
}

type LogoTypeMsg struct {
	Logo LogoType `struc:"int32,little"`
}

type DisconnectPhone struct {
}

type CloseDongle struct {
}

type SendIconConfig struct {
	Config []byte `struc:"skip"`
}
