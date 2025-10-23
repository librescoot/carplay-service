package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"reflect"

	"github.com/lunixbochs/struc"
)

const magicNumber uint32 = 0x55aa55aa

var messageTypes = map[reflect.Type]uint32{
	reflect.TypeOf(&SendFile{}):            0x99,
	reflect.TypeOf(&Open{}):                0x01,
	reflect.TypeOf(&Opened{}):              0x01,
	reflect.TypeOf(&Heartbeat{}):           0xaa,
	reflect.TypeOf(&ManufacturerInfo{}):    0x14,
	reflect.TypeOf(&CarPlay{}):             0x08,
	reflect.TypeOf(&SoftwareVersion{}):     0xcc,
	reflect.TypeOf(&BluetoothAddress{}):    0x0a,
	reflect.TypeOf(&BluetoothPIN{}):        0x0c,
	reflect.TypeOf(&Plugged{}):             0x02,
	reflect.TypeOf(&Unplugged{}):           0x04,
	reflect.TypeOf(&VideoData{}):           0x06,
	reflect.TypeOf(&AudioData{}):           0x07,
	reflect.TypeOf(&Touch{}):               0x05,
	reflect.TypeOf(&BluetoothDeviceName{}): 0x0d,
	reflect.TypeOf(&WifiDeviceName{}):      0x0e,
	reflect.TypeOf(&BluetoothPairedList{}): 0x12,
	reflect.TypeOf(&MultiTouch{}):          0x17,
	reflect.TypeOf(&Phase{}):               0x03,
	reflect.TypeOf(&HiCarLink{}):           0x18,
	reflect.TypeOf(&BoxSettings{}):         0x19,
	reflect.TypeOf(&MediaData{}):           0x2a,
	reflect.TypeOf(&LogoTypeMsg{}):         0x09,
	reflect.TypeOf(&DisconnectPhone{}):     0x0f,
	reflect.TypeOf(&CloseDongle{}):         0x15,
}

// Header is header structure of data protocol
type Header struct {
	Magic  uint32 `struc:"uint32,little"`
	Length uint32 `struc:"uint32,little"`
	Type   uint32 `struc:"uint32,little"`
	TypeN  uint32 `struc:"uint32,little"`
}

func packPayload(buffer io.Writer, payload interface{}) error {
	if reflect.ValueOf(payload).Elem().NumField() > 0 {
		return struc.Pack(buffer, payload)
	}
	// Nothing to do
	return nil
}

func packHeader(payload interface{}, buffer io.Writer, data []byte) error {
	msgType, found := messageTypes[reflect.TypeOf(payload)]
	if !found {
		return errors.New("No message found")
	}
	msgTypeN := (msgType ^ 0xffffffff) & 0xffffffff
	msg := &Header{Magic: magicNumber, Length: uint32(len(data)), Type: msgType, TypeN: msgTypeN}
	err := struc.Pack(buffer, msg)
	if err != nil {
		return err
	}
	_, err = buffer.Write(data)
	return err
}

func Marshal(payload interface{}) ([]byte, error) {
	var buf, buffer bytes.Buffer
	err := packPayload(&buf, payload)
	if err != nil {
		return nil, err
	}
	err = packHeader(payload, &buffer, buf.Bytes())
	return buffer.Bytes(), err
}

func GetPayloadByHeader(hdr Header) interface{} {
	for key, value := range messageTypes {
		if value == hdr.Type {
			return reflect.New(key.Elem()).Interface()
		}
	}
	return &Unknown{Type: hdr.Type}
}

func Unmarshal(data []byte, payload interface{}) error {
	if len(data) > 0 {
		err := struc.Unpack(bytes.NewBuffer(data), payload)
		if err != nil {
			return err
		}
	}

	switch payload := payload.(type) {
	case *Header:
		if payload.Magic != magicNumber {
			return errors.New("Invalid magic number")
		}
		if (payload.Type^0xffffffff)&0xffffffff != payload.TypeN {
			return errors.New("Invalid type")
		}
	case *AudioData:
		switch len(data) - 12 {
		case 1:
			payload.Command = AudioCommand(data[12])
		case 4:
			binary.Read(bytes.NewBuffer(data[12:]), binary.LittleEndian, &payload.VolumeDuration)
		default:
			payload.Data = data[12:]
		}
	case *VideoData:
		// VideoData structure: Width(4) + Height(4) + Flags(4) + Length(4) + Unknown2(4) = 20 bytes header
		// The actual H.264 video frame data starts at byte offset 20
		// This matches the TypeScript implementation: data.subarray(20)
		if len(data) >= 20 {
			// Parse fields directly from byte slice (fast)
			payload.Width = int32(binary.LittleEndian.Uint32(data[0:4]))
			payload.Height = int32(binary.LittleEndian.Uint32(data[4:8]))
			payload.Flags = int32(binary.LittleEndian.Uint32(data[8:12]))
			payload.Length = int32(binary.LittleEndian.Uint32(data[12:16]))
			payload.Unknown2 = int32(binary.LittleEndian.Uint32(data[16:20]))
			// CRITICAL: Copy the video frame data instead of using slice reference
			// If we just do data[20:], the underlying buffer can be reused/corrupted
			// This causes "non-existing PPS" errors in ffmpeg
			if len(data) > 20 {
				payload.Data = make([]byte, len(data)-20)
				copy(payload.Data, data[20:])
			}
		}
	case *BluetoothDeviceName:
		payload.Data = NullTermString(data)
	case *WifiDeviceName:
		payload.Data = NullTermString(data)
	case *BluetoothPairedList:
		payload.Data = NullTermString(data)
	case *HiCarLink:
		payload.Link = NullTermString(data)
	case *BoxSettings:
		payload.Settings = data
	case *MediaData:
		if len(data) > 4 {
			payload.MediaInfo = data[4:]
		}
	case *MultiTouch:
		// Parse multiple touch items
		itemSize := 16 // 4 floats * 4 bytes
		numTouches := len(data) / itemSize
		payload.Touches = make([]TouchItem, numTouches)
		for i := 0; i < numTouches; i++ {
			offset := i * itemSize
			item := &payload.Touches[i]
			binary.Read(bytes.NewBuffer(data[offset:offset+4]), binary.LittleEndian, &item.X)
			binary.Read(bytes.NewBuffer(data[offset+4:offset+8]), binary.LittleEndian, &item.Y)
			binary.Read(bytes.NewBuffer(data[offset+8:offset+12]), binary.LittleEndian, &item.Action)
			binary.Read(bytes.NewBuffer(data[offset+12:offset+16]), binary.LittleEndian, &item.ID)
		}
	case *Unknown:
		payload.Data = data
	}

	return nil
}
