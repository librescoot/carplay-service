package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/mzyy94/gocarplay"
	"github.com/mzyy94/gocarplay/link"
	"github.com/mzyy94/gocarplay/protocol"
	redisClient "github.com/mzyy94/gocarplay/redis"
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
	size        deviceSize
	fps         int32 = 30 // Output fps after ffmpeg conversion
	dongleReady bool
	debugMode   bool // Enable verbose debug logging via DEBUG=1 environment variable

	// MJPEG streaming
	streamClients sync.Map // map of client channels
	jpegFrames    chan []byte
	h264Frames    chan []byte

	// ffmpeg process
	ffmpegCmd           *exec.Cmd
	ffmpegStdin         io.WriteCloser
	ffmpegStdout        io.ReadCloser
	ffmpegStdoutBuffered *bufio.Reader // Buffered reader for efficient I/O
	ffmpegMutex         sync.Mutex

	// Debug counters
	h264FrameCount int64
	jpegFrameCount int64

	// Frame size statistics for diagnostic purposes
	h264TotalSize int64
	jpegTotalSize int64

	// Redis client for state publishing
	redis *redisClient.Client
)

func init() {
	// Minimal buffers for low latency
	// 3 frames = ~100ms at 30fps
	// Small buffers are critical to avoid lag - we want real-time streaming!
	jpegFrames = make(chan []byte, 3)
	h264Frames = make(chan []byte, 3)
}

// mapDeviceType converts protocol.PhoneType to simple device type string
func mapDeviceType(phoneType protocol.PhoneType) string {
	switch phoneType {
	case protocol.AndroidAuto, protocol.AndroidMirror, protocol.HiCar:
		return "android"
	case protocol.PhoneTypeCarPlay, protocol.IPhoneMirror:
		return "ios"
	default:
		return "unknown"
	}
}

func initializeDongle() error {
	size.Width = 800
	size.Height = 480

	log.Println("Initializing USB connection to dongle...")
	if err := link.Init(); err != nil {
		return fmt.Errorf("failed to initialize link: %v", err)
	}

	config := gocarplay.DefaultConfig()
	config.Width = 800
	config.Height = 480
	config.Fps = 30  // Request 30fps from dongle instead of 60fps
	config.Dpi = 140
	config.AudioTransferMode = false // Disable audio transfer - we don't use it

	log.Println("Starting communication with dongle...")
	go link.Communicate(func(data interface{}) {
		switch data := data.(type) {
		case *protocol.VideoData:
			// Send H.264 frame to converter
			h264FrameCount++

			// Warn about suspiciously small H.264 frames (likely black/empty content)
			if len(data.Data) > 0 && len(data.Data) < 100 && h264FrameCount > 10 {
				if h264FrameCount%100 == 0 {
					log.Printf("[Video] WARNING: Frame #%d is very small (%d bytes) - may be black/empty content",
						h264FrameCount, len(data.Data))
				}
			}

			// Comprehensive diagnostic for first 10 frames
			if h264FrameCount <= 10 && len(data.Data) >= 5 {
				nalType := "unknown"
				if len(data.Data) >= 5 && data.Data[0] == 0 && data.Data[1] == 0 {
					nalUnitType := data.Data[4] & 0x1F
					switch nalUnitType {
					case 1:
						nalType = "P-frame (non-IDR)"
					case 5:
						nalType = "I-frame (IDR)"
					case 6:
						nalType = "SEI"
					case 7:
						nalType = "SPS"
					case 8:
						nalType = "PPS"
					case 9:
						nalType = "Access Unit Delimiter"
					default:
						nalType = fmt.Sprintf("type_%d", nalUnitType)
					}
				}

				hexDump := ""
				dumpLen := 32
				if len(data.Data) < dumpLen {
					dumpLen = len(data.Data)
				}
				for i := 0; i < dumpLen; i++ {
					hexDump += fmt.Sprintf("%02x ", data.Data[i])
				}

				log.Printf("[Video] Frame #%d: NAL=%s, Size=%d, Flags=%d, First32bytes: %s",
					h264FrameCount, nalType, len(data.Data), data.Flags, hexDump)
			}

			if debugMode && h264FrameCount%500 == 1 {
				log.Printf("[Video] Frame #%d: Width=%d, Height=%d, Flags=%d, Length=%d, DataSize=%d",
					h264FrameCount, data.Width, data.Height, data.Flags, data.Length, len(data.Data))
			}

			// Only send non-empty frames
			if len(data.Data) > 0 {
				// CRITICAL: Drain old frames to always prioritize the LATEST frame
				// This prevents lag buildup - we'd rather drop old frames than queue them
				drained := 0
				drainLoop:
					for {
						select {
						case <-h264Frames:
							drained++
						default:
							break drainLoop
						}
					}

				if drained > 0 && debugMode {
					log.Printf("[Video] Drained %d old H.264 frames to prioritize latest", drained)
				}

				// Now send the latest frame (non-blocking)
				select {
				case h264Frames <- data.Data:
				default:
					// Still full? Drop this frame too
					if debugMode {
						log.Printf("[Video] WARNING: Dropped H.264 frame, channel full after drain")
					}
				}
			}
		case *protocol.Plugged:
			log.Printf("[Device Plugged] PhoneType: %v, WiFi: %v", data.PhoneType, data.Wifi)
			link.HandlePhonePlugged(data)

			// Publish device connection to Redis
			if redis != nil {
				redis.PublishState("device_connected", "true")
				redis.PublishState("device_type", mapDeviceType(data.PhoneType))
			}
		case *protocol.Unplugged:
			log.Println("[Device Unplugged]")

			// Publish device disconnection to Redis
			if redis != nil {
				redis.PublishState("device_connected", "false")
				redis.PublishState("device_type", "none")
			}
		case *protocol.Phase:
			log.Printf("[Phase] %v", data.PhaseValue)
		case *protocol.BoxSettings:
			log.Printf("[BoxSettings] %s", string(data.Settings))
		case *protocol.MediaData:
			handleMediaData(data)
		case *protocol.AudioData:
			// Silently drop audio data - we're not using it
			// Only log in debug mode
			if debugMode {
				log.Printf("[Audio] Received %d bytes (DecodeType: %d, AudioType: %d)",
					len(data.Data), data.DecodeType, data.AudioType)
			}
		default:
			log.Printf("[onData] %#v", data)
		}
	}, func(err error) {
		log.Printf("[ERROR] %#v", err)

		// Publish error to Redis
		if redis != nil {
			redis.PublishState("error", fmt.Sprintf("%v", err))
		}
	})

	go link.StartWithConfig(config)

	// Start ffmpeg converter
	if err := startFFmpegConverter(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %v", err)
	}

	// Give ffmpeg time to initialize its decoder before frames arrive
	// This prevents "Invalid data" errors when first frames arrive too early
	time.Sleep(200 * time.Millisecond)
	log.Println("[FFmpeg] Initialization delay complete, ready to process frames")

	// Start frame broadcaster
	go broadcastFrames()

	dongleReady = true
	log.Println("Dongle initialized successfully and ready for connections")

	// Publish dongle availability to Redis
	if redis != nil {
		redis.PublishState("dongle_available", "true")
		redis.PublishState("error", "")
	}

	return nil
}

func handleMediaData(data *protocol.MediaData) {
	if data.Type == protocol.MediaTypeData {
		var mediaInfo map[string]interface{}
		if err := json.Unmarshal(data.MediaInfo, &mediaInfo); err == nil {
			log.Printf("[Media Info] %v", mediaInfo)
		}
	} else if data.Type == protocol.MediaTypeAlbumCover {
		log.Printf("[Album Cover] Received %d bytes of image data", len(data.MediaInfo))
	}
}

func startFFmpegConverter() error {
	ffmpegMutex.Lock()
	defer ffmpegMutex.Unlock()

	// ffmpeg command: read H.264 from stdin, output JPEG frames to stdout
	// -f h264 explicitly tells ffmpeg the input is raw H.264 Annex-B stream
	ffmpegCmd = exec.Command("ffmpeg",
		"-f", "h264",
		"-threads", "4",
		"-i", "pipe:0",
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"-q:v", "5",
		"pipe:1",
	)

	var err error
	ffmpegStdin, err = ffmpegCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %v", err)
	}

	ffmpegStdout, err = ffmpegCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %v", err)
	}

	// Wrap stdout with a large buffered reader (256KB) for efficient I/O
	// This reduces kernel overhead and improves streaming performance
	ffmpegStdoutBuffered = bufio.NewReaderSize(ffmpegStdout, 256*1024)

	// Capture stderr for debugging
	stderrPipe, err := ffmpegCmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %v", err)
	}

	if err := ffmpegCmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %v", err)
	}

	log.Println("[FFmpeg] Started converter process with 256KB buffered I/O")

	// Log ffmpeg errors in background
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			// Only log important messages, skip verbose output
			if len(line) > 0 && line[0] != ' ' {
				log.Printf("[FFmpeg] %s", line)
			}
		}
	}()

	// Start H.264 writer goroutine
	go writeH264ToFFmpeg()

	// Start JPEG reader goroutine
	go readJPEGFromFFmpeg()

	return nil
}

func writeH264ToFFmpeg() {
	for frame := range h264Frames {
		// Get pipe reference with mutex, but don't hold it during blocking write
		// This prevents deadlock when ffmpeg blocks on stdin
		ffmpegMutex.Lock()
		stdin := ffmpegStdin
		ffmpegMutex.Unlock()

		if stdin != nil {
			if _, err := stdin.Write(frame); err != nil {
				log.Printf("[FFmpeg] Error writing H.264 frame: %v", err)
				continue
			}
		}
	}
}

func readJPEGFromFFmpeg() {
	for {
		// Read JPEG frame from buffered ffmpeg stdout
		// JPEG format: starts with FF D8, ends with FF D9
		jpeg, err := readJPEGFrame(ffmpegStdoutBuffered)
		if err != nil {
			log.Printf("[FFmpeg] Error reading JPEG frame: %v", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}

		jpegFrameCount++

		// Validate JPEG frame markers
		if jpegFrameCount <= 5 || jpegFrameCount%100 == 0 {
			hasValidStart := len(jpeg) >= 2 && jpeg[0] == 0xFF && jpeg[1] == 0xD8
			hasValidEnd := len(jpeg) >= 2 && jpeg[len(jpeg)-2] == 0xFF && jpeg[len(jpeg)-1] == 0xD9
			log.Printf("[JPEG] Frame #%d: size=%d, validStart=%v, validEnd=%v, first4bytes=[%02X %02X %02X %02X]",
				jpegFrameCount, len(jpeg), hasValidStart, hasValidEnd,
				jpeg[0], jpeg[1], jpeg[2], jpeg[3])
		}

		if debugMode && jpegFrameCount%500 == 1 {
			log.Printf("[FFmpeg] Converted JPEG frame #%d, size: %d bytes", jpegFrameCount, len(jpeg))
		}

		// CRITICAL: Drain old JPEG frames to prioritize the latest
		drained := 0
		drainJpegLoop:
			for {
				select {
				case <-jpegFrames:
					drained++
				default:
					break drainJpegLoop
				}
			}

		if drained > 0 && debugMode {
			log.Printf("[FFmpeg] Drained %d old JPEG frames to prioritize latest", drained)
		}

		// Send to broadcast channel (non-blocking)
		select {
		case jpegFrames <- jpeg:
		default:
			// Drop frame if buffer is full (only log in debug mode)
			if debugMode {
				log.Printf("[FFmpeg] WARNING: Dropped JPEG frame, channel full after drain")
			}
		}
	}
}

func readJPEGFrame(reader io.Reader) ([]byte, error) {
	// SIMPLE BYTE-BY-BYTE READING - most reliable approach
	// Testing version to isolate if buffered reading was causing issues

	var buf bytes.Buffer
	b := make([]byte, 1)

	// Find JPEG start marker (FF D8)
	for {
		if _, err := reader.Read(b); err != nil {
			return nil, err
		}
		if b[0] == 0xFF {
			if _, err := reader.Read(b); err != nil {
				return nil, err
			}
			if b[0] == 0xD8 {
				// Found start marker
				buf.Write([]byte{0xFF, 0xD8})
				break
			}
		}
	}

	// Read until we find end marker (FF D9)
	prevByte := byte(0)
	for {
		if _, err := reader.Read(b); err != nil {
			return nil, err
		}
		buf.WriteByte(b[0])

		if prevByte == 0xFF && b[0] == 0xD9 {
			// Found end marker, JPEG is complete
			return buf.Bytes(), nil
		}
		prevByte = b[0]
	}
}

// Broadcast JPEG frames to all connected clients
func broadcastFrames() {
	for frame := range jpegFrames {
		streamClients.Range(func(key, value interface{}) bool {
			clientChan := value.(chan []byte)

			// Drain old frames from this client's channel to prioritize latest
			drained := 0
		drainClientLoop:
			for {
				select {
				case <-clientChan:
					drained++
				default:
					break drainClientLoop
				}
			}

			// Now send the latest frame (non-blocking)
			select {
			case clientChan <- frame:
			default:
				// Skip if client is still slow after draining
			}
			return true
		})
	}
}

func streamHandler(w http.ResponseWriter, r *http.Request) {
	if !dongleReady {
		http.Error(w, "Dongle not ready", http.StatusServiceUnavailable)
		return
	}

	// Set headers for MJPEG streaming
	boundary := "frame"
	w.Header().Set("Content-Type", fmt.Sprintf("multipart/x-mixed-replace; boundary=%s", boundary))
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	clientID := fmt.Sprintf("%p", r)
	log.Printf("[Stream] New MJPEG client connected: %s", clientID)

	// Create channel for this client with minimal buffer for low latency
	// 2 frames = ~66ms at 30fps
	clientChan := make(chan []byte, 2)
	streamClients.Store(clientID, clientChan)

	// Clean up on disconnect
	defer func() {
		streamClients.Delete(clientID)
		close(clientChan)
		log.Printf("[Stream] Client disconnected: %s", clientID)
	}()

	// Create multipart writer
	mw := multipart.NewWriter(w)
	mw.SetBoundary(boundary)

	// Stream JPEG frames to client
	framesSent := int64(0)
	for {
		select {
		case frame := <-clientChan:
			framesSent++

			// Log first 5 frames and every 100th frame
			if framesSent <= 5 || framesSent%100 == 0 {
				log.Printf("[Stream] Sending frame #%d to client %s: size=%d bytes",
					framesSent, clientID, len(frame))
			}

			// Create part header
			partHeader := make(textproto.MIMEHeader)
			partHeader.Add("Content-Type", "image/jpeg")
			partHeader.Add("Content-Length", fmt.Sprintf("%d", len(frame)))

			part, err := mw.CreatePart(partHeader)
			if err != nil {
				log.Printf("[Stream] Error creating part: %v", err)
				return
			}

			// Write JPEG data
			if _, err := part.Write(frame); err != nil {
				log.Printf("[Stream] Error writing frame: %v", err)
				return
			}

			flusher.Flush()

		case <-r.Context().Done():
			return
		}
	}
}

func touchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var touch deviceTouch
	if err := json.NewDecoder(r.Body).Decode(&touch); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Convert coordinates and send to dongle
	x := uint32(touch.X * 10000 / float32(size.Width))
	y := uint32(touch.Y * 10000 / float32(size.Height))

	if err := link.SendData(&protocol.Touch{
		X:      x,
		Y:      y,
		Action: protocol.TouchAction(touch.Action),
	}); err != nil {
		log.Printf("[Touch] Error sending touch event: %v", err)
		http.Error(w, "Failed to send touch event", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var clientCount int
	streamClients.Range(func(key, value interface{}) bool {
		clientCount++
		return true
	})

	if dongleReady {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ready",
			"width":   size.Width,
			"height":  size.Height,
			"fps":     fps,
			"clients": clientCount,
			"format":  "MJPEG",
		})
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "initializing",
		})
	}
}

func cleanup() {
	log.Println("Shutting down...")

	// Publish dongle unavailable state before shutdown
	if redis != nil {
		redis.PublishState("dongle_available", "false")
		redis.PublishState("device_connected", "false")
		redis.PublishState("device_type", "none")
		redis.Close()
	}

	ffmpegMutex.Lock()
	defer ffmpegMutex.Unlock()

	if ffmpegStdin != nil {
		ffmpegStdin.Close()
	}
	if ffmpegCmd != nil && ffmpegCmd.Process != nil {
		ffmpegCmd.Process.Kill()
		ffmpegCmd.Wait()
	}
}

func main() {
	// Check for debug mode
	debugMode = os.Getenv("DEBUG") == "1"

	log.Println("GoCarPlay Server starting...")
	log.Println("Configured resolution: 800x480 @ 30fps")
	log.Println("Streaming: MJPEG over HTTP (H.264 -> JPEG conversion via ffmpeg)")
	if debugMode {
		log.Println("Debug mode: ENABLED (set DEBUG=0 to disable)")
	} else {
		log.Println("Debug mode: DISABLED (set DEBUG=1 to enable)")
	}

	// Initialize Redis client
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}
	redis = redisClient.NewClient(redisAddr)
	log.Printf("Redis client initialized (address: %s)", redisAddr)

	// Test Redis connection (non-fatal if it fails)
	if err := redis.Ping(); err != nil {
		log.Printf("Warning: Redis connection failed: %v (continuing without Redis)", err)
	} else {
		log.Println("Redis connection successful")
	}

	// Initialize dongle on startup
	if err := initializeDongle(); err != nil {
		// Publish dongle failure to Redis before exiting
		if redis != nil {
			redis.PublishState("dongle_available", "false")
			redis.PublishState("error", fmt.Sprintf("Dongle initialization failed: %v", err))
		}
		log.Fatalf("Failed to initialize dongle: %v", err)
	}

	// Wait a moment for initialization to complete
	time.Sleep(2 * time.Second)

	// Setup HTTP endpoints
	http.HandleFunc("/touch", touchHandler)
	http.HandleFunc("/stream", streamHandler)
	http.HandleFunc("/status", statusHandler)

	log.Println("Server ready on http://localhost:8001")
	log.Println("Endpoints:")
	log.Println("  POST /touch  - Touch input endpoint")
	log.Println("  GET  /stream - MJPEG video stream")
	log.Println("  GET  /status - Health check endpoint")

	// Cleanup on exit
	defer cleanup()

	log.Fatal(http.ListenAndServe(":8001", nil))
}
