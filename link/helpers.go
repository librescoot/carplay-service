package link

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mzyy94/gocarplay/protocol"
)

// SendMultiTouch sends a multi-touch message with multiple touch points
func SendMultiTouch(touches []protocol.TouchItem) error {
	// Build the touches data
	var buf bytes.Buffer
	for _, touch := range touches {
		binary.Write(&buf, binary.LittleEndian, touch.X)
		binary.Write(&buf, binary.LittleEndian, touch.Y)
		binary.Write(&buf, binary.LittleEndian, touch.Action)
		binary.Write(&buf, binary.LittleEndian, touch.ID)
	}

	return SendData(&protocol.MultiTouch{Touches: touches})
}

// SendSingleTouch sends a single touch event
func SendSingleTouch(x, y float32, action protocol.TouchAction) error {
	// Convert to 0-10000 scale as expected by the protocol
	scaledX := uint32(x * 10000)
	scaledY := uint32(y * 10000)

	// Clamp values
	if scaledX > 10000 {
		scaledX = 10000
	}
	if scaledY > 10000 {
		scaledY = 10000
	}

	return SendData(&protocol.Touch{
		Action: action,
		X:      scaledX,
		Y:      scaledY,
		Flags:  0,
	})
}

// SendMediaInfo sends media information (song, album, artist, etc.)
func SendMediaInfo(songName, albumName, artistName, appName string, duration, playTime int) error {
	mediaInfo := map[string]interface{}{
		"MediaSongName":     songName,
		"MediaAlbumName":    albumName,
		"MediaArtistName":   artistName,
		"MediaAPPName":      appName,
		"MediaSongDuration": duration,
		"MediaSongPlayTime": playTime,
	}

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(protocol.MediaTypeData))

	// Convert media info to JSON and append
	jsonData, err := json.Marshal(mediaInfo)
	if err != nil {
		return err
	}
	buf.Write(jsonData)
	buf.WriteByte(0) // Null terminator

	return SendData(&protocol.MediaData{
		Type:      protocol.MediaTypeData,
		MediaInfo: buf.Bytes()[4:],
	})
}

// SendAlbumCover sends album cover image data
func SendAlbumCover(imageData []byte) error {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(protocol.MediaTypeAlbumCover))
	buf.Write(imageData)

	return SendData(&protocol.MediaData{
		Type:      protocol.MediaTypeAlbumCover,
		MediaInfo: imageData,
	})
}

// SendLogoType sends a logo type configuration message
func SendLogoType(logoType protocol.LogoType) error {
	return SendData(&protocol.LogoTypeMsg{Logo: logoType})
}

// SendIconConfig sends icon configuration
func SendIconConfig(label string) error {
	config := map[string]interface{}{
		"oemIconVisible": 1,
		"name":           "AutoBox",
		"model":          "GoCarPlay-1.00",
		"oemIconPath":    string(protocol.FileAddressOEMIcon),
	}

	if label != "" {
		config["oemIconLabel"] = label
	}

	var lines []string
	for k, v := range config {
		lines = append(lines, fmt.Sprintf("%s = %v", k, v))
	}
	configData := strings.Join(lines, "\n") + "\n"

	return SendData(&protocol.SendFile{
		FileName: protocol.NullTermString(protocol.FileAddressAirplayConfig + "\x00"),
		Content:  []byte(configData),
	})
}

// SendNightMode enables or disables night mode
func SendNightMode(enable bool) error {
	if enable {
		return SendCommand(protocol.EnableNightMode)
	}
	return SendCommand(protocol.DisableNightMode)
}

// SendPhoneCallAction sends phone call accept/reject commands
func SendPhoneCallAction(accept bool) error {
	if accept {
		return SendCommand(protocol.AcceptPhoneCall)
	}
	return SendCommand(protocol.RejectPhoneCall)
}

// SendVideoFocus requests or releases video focus
func SendVideoFocus(request bool) error {
	if request {
		return SendCommand(protocol.RequestVideoFocus)
	}
	return SendCommand(protocol.ReleaseVideoFocus)
}