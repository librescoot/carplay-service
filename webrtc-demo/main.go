package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"

	"github.com/mzyy94/gocarplay"
	"github.com/mzyy94/gocarplay/link"
	"github.com/mzyy94/gocarplay/protocol"
)

type deviceSize struct {
	Width  int32 `json:"width"`
	Height int32 `json:"height"`
}

type deviceTouch struct {
	X      float32 `json:"x"`
	Y      float32 `json:"y"`
	Action int32   `json:"action"`
}

var (
	videoTrack       *webrtc.TrackLocalStaticSample
	audioDataChannel *webrtc.DataChannel
	size             deviceSize
	fps              int32 = 25
)

func setupWebRTC(offer webrtc.SessionDescription) (*webrtc.SessionDescription, error) {
	// WebRTC setup
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}
	mediaEngine := webrtc.MediaEngine{}

	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(&mediaEngine))

	pc, err := api.NewPeerConnection(config)
	if err != nil {
		return nil, err
	}

	stats, ok := pc.GetStats().GetConnectionStats(pc)
	if !ok {
		stats.ID = "unknoown"
	}

	pc.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		log.Printf("State of %s: %s \n", stats.ID, connectionState.String())
	})

	// Create a video track
	videoCodec := webrtc.RTPCodecCapability{
		MimeType:     webrtc.MimeTypeH264,
		ClockRate:    90000,
		Channels:     0,
		SDPFmtpLine:  "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=640032",
		RTCPFeedback: nil,
	}
	if videoTrack, err = webrtc.NewTrackLocalStaticSample(videoCodec, "video", "video"); err != nil {
		return nil, err
	}

	if _, err = pc.AddTransceiverFromTrack(videoTrack,
		webrtc.RtpTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionSendonly,
		},
	); err != nil {
		return nil, err
	}

	// Create a data channels
	audioDataChannel, err = pc.CreateDataChannel("audio", nil)
	if err != nil {
		return nil, err
	}

	pc.OnDataChannel(func(d *webrtc.DataChannel) {
		switch d.Label() {
		case "touch":
			d.OnMessage(func(msg webrtc.DataChannelMessage) {
				sendTouch(msg.Data)
			})
		case "start":
			d.OnMessage(func(msg webrtc.DataChannelMessage) {
				startCarPlay(msg.Data)
			})
		}
	})

	// Set the remote SessionDescription
	if err := pc.SetRemoteDescription(offer); err != nil {
		return nil, err
	}

	// Create an answer
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	if err = pc.SetLocalDescription(answer); err != nil {
		return nil, err
	}

	return &answer, nil
}

func webRTCOfferHandler(w http.ResponseWriter, r *http.Request) {
	var offer webrtc.SessionDescription
	if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "{\"error\": \"%s\"}", err.Error())
		return
	}

	answer, err := setupWebRTC(offer)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "{\"error\": \"%s\"}", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(&answer)
}

func sendTouch(data []byte) {
	var touch deviceTouch
	if err := json.Unmarshal(data, &touch); err != nil {
		return
	}

	link.SendData(&protocol.Touch{X: uint32(touch.X * 10000 / float32(size.Width)), Y: uint32(touch.Y * 10000 / float32(size.Height)), Action: protocol.TouchAction(touch.Action)})
}

func startCarPlay(data []byte) {
	type startConfig struct {
		Width     int32  `json:"width"`
		Height    int32  `json:"height"`
		Fps       int32  `json:"fps,omitempty"`
		Dpi       int32  `json:"dpi,omitempty"`
		NightMode bool   `json:"nightMode,omitempty"`
		WifiType  string `json:"wifiType,omitempty"`
		MicType   string `json:"micType,omitempty"`
	}

	var cfg startConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("Failed to parse config: %v", err)
		return
	}

	// Update global size
	size.Width = cfg.Width
	size.Height = cfg.Height

	// Set FPS if provided
	if cfg.Fps > 0 {
		fps = cfg.Fps
	}

	if err := link.Init(); err != nil {
		log.Printf("Failed to initialize link: %v", err)
		return
	}

	// Create configuration with user parameters
	config := gocarplay.DefaultConfig()
	config.Width = cfg.Width
	config.Height = cfg.Height
	config.Fps = fps
	if cfg.Dpi > 0 {
		config.Dpi = cfg.Dpi
	}
	if cfg.WifiType != "" {
		config.WifiType = cfg.WifiType
	}
	if cfg.MicType != "" {
		config.MicType = cfg.MicType
	}
	config.NightMode = cfg.NightMode

	go link.Communicate(func(data interface{}) {
		switch data := data.(type) {
		case *protocol.VideoData:
			duration := time.Duration((float32(1) / float32(fps)) * float32(time.Second))
			videoTrack.WriteSample(media.Sample{Data: data.Data, Duration: duration})
		case *protocol.AudioData:
			if len(data.Data) == 0 {
				handleAudioCommand(data)
			} else {
				var buf bytes.Buffer
				fr := protocol.AudioDecodeTypes[data.DecodeType].Frequency
				ch := protocol.AudioDecodeTypes[data.DecodeType].Channel
				binary.Write(&buf, binary.LittleEndian, fr)
				binary.Write(&buf, binary.LittleEndian, ch)
				audioDataChannel.Send(append(buf.Bytes(), data.Data...))
			}
		case *protocol.MediaData:
			handleMediaData(data)
		case *protocol.Plugged:
			log.Printf("[Device Plugged] PhoneType: %v, WiFi: %v", data.PhoneType, data.Wifi)
			// Automatically enable Android work mode if Android device is detected
			link.HandlePhonePlugged(data)
		case *protocol.Phase:
			log.Printf("[Phase] %v", data.PhaseValue)
		case *protocol.BoxSettings:
			log.Printf("[BoxSettings] %s", string(data.Settings))
		default:
			log.Printf("[onData] %#v", data)
		}
	}, func(err error) {
		log.Printf("[ERROR] %#v", err)
	})

	// Start with configuration
	go link.StartWithConfig(config)
}

func handleAudioCommand(data *protocol.AudioData) {
	if data.Command != 0 {
		log.Printf("[Audio Command] %v", data.Command)
		// Handle audio commands like Siri start/stop, media start/stop, etc.
		switch data.Command {
		case protocol.AudioSiriStart:
			log.Println("Siri started")
		case protocol.AudioSiriStop:
			log.Println("Siri stopped")
		case protocol.AudioMediaStart:
			log.Println("Media playback started")
		case protocol.AudioMediaStop:
			log.Println("Media playback stopped")
		}
	}
}

func handleMediaData(data *protocol.MediaData) {
	if data.Type == protocol.MediaTypeData {
		// Parse media info JSON
		var mediaInfo map[string]interface{}
		if err := json.Unmarshal(data.MediaInfo, &mediaInfo); err == nil {
			log.Printf("[Media Info] %v", mediaInfo)
		}
	} else if data.Type == protocol.MediaTypeAlbumCover {
		log.Printf("[Album Cover] Received %d bytes of image data", len(data.MediaInfo))
	}
}

func main() {
	log.Println("http://localhost:8001")
	http.HandleFunc("/connect", webRTCOfferHandler)
	http.Handle("/", http.FileServer(http.Dir("./")))
	log.Fatal(http.ListenAndServe(":8001", nil))
}
