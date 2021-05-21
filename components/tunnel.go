package components

import (
	"encoding/json"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/sacOO7/gowebsocket"
)

/**
 * Created by Aditya Awasthi on 04/02/2021.
 * @author github.com/adwardstark
 */

// Envelope defines the structure of
// a JSON message travelling through the tunnel
type envelope struct {
	Type      string      `json:"type"`
	SessionID string      `json:"sessionID"`
	Payload   interface{} `json:"payload"`
}

const (
	typeInput              = "input"
	typeOutput             = "output"
	typeResize             = "resize"
	typeStart              = "start"
	typeEnd                = "end"
	errInvalidEnvelope     = "Data could not be parsed as JSON"
	errInvalidObjectFormat = "Object format invalid"
)

func isValidJSON(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

// SocketTunnel defines structure of the tunnel and callbacks
type SocketTunnel struct {
	socket        gowebsocket.Socket
	reconnectWait int
	OnError       func(err error)
	OnStart       func(sessionID string)
	OnEnd         func(sessionID string)
	OnInput       func(sessionID string, payload string)
	OnResize      func(sessionID string, width int64, height int64)
}

// NewTunnel returns a new instance of SocketTunnel
func NewTunnel(url string) SocketTunnel {
	return SocketTunnel{
		socket:        gowebsocket.New(url),
		reconnectWait: 1,
	}
}

// StartTunnel will register callbacks and start connection
func (tunnel *SocketTunnel) StartTunnel() {
	tunnel.socket.ConnectionOptions = gowebsocket.ConnectionOptions{
		UseSSL:         false, // Don't use SSL
		UseCompression: false, // Don't use compression
	}

	tunnel.socket.OnConnected = func(socket gowebsocket.Socket) {
		log.Printf("Tunnel connected at: %s\n", socket.Url)
		tunnel.reconnectWait = 1
	}
	tunnel.socket.OnDisconnected = func(err error, socket gowebsocket.Socket) {
		log.Println("Tunnel disconnected")
		tunnel.OnError(err)
		handleConnection(tunnel)
	}
	tunnel.socket.OnConnectError = func(err error, socket gowebsocket.Socket) {
		tunnel.OnError(err)
	}
	tunnel.socket.OnTextMessage = func(message string, socket gowebsocket.Socket) {
		if ok := isValidJSON(message); !ok {
			log.Printf("%s,\n%s", errInvalidEnvelope, message)
			return
		}

		var envelope envelope
		decoder := json.NewDecoder(strings.NewReader(message))
		decoder.UseNumber()

		// Disallow unknown fields to validate data
		decoder.DisallowUnknownFields()

		err := decoder.Decode(&envelope)
		if err != nil {
			log.Printf("%s,\n%s", errInvalidObjectFormat, message)
			return
		}

		switch envelope.Type {
		case typeResize:
			resize, ok := envelope.Payload.(map[string]interface{})
			if !ok {
				log.Printf("%s,\n%s", errInvalidObjectFormat, message)
				return
			}

			width, widthOK := resize["width"]
			height, heightOK := resize["height"]
			if !widthOK || !heightOK {
				log.Printf("%s,\n%s", errInvalidObjectFormat, message)
				return
			}

			w, ok := width.(json.Number)
			if !ok {
				log.Printf("%s,\n%s", errInvalidObjectFormat, message)
				return
			}

			intWidth, err := w.Int64()
			if err != nil || intWidth < 0 {
				log.Printf("%s,\n%s", errInvalidObjectFormat, message)
				return
			}

			h, ok := height.(json.Number)
			if !ok {
				log.Printf("%s,\n%s", errInvalidObjectFormat, message)
				return
			}

			intHeight, err := h.Int64()
			if err != nil || intHeight < 0 {
				log.Printf("%s,\n%s", errInvalidObjectFormat, message)
				return
			}

			tunnel.OnResize(envelope.SessionID, intWidth, intHeight)
		case typeInput:
			// Validate payload type
			if reflect.TypeOf(envelope.Payload) == nil || reflect.TypeOf(envelope.Payload).Name() != "string" {
				log.Printf("%s,\n%s", errInvalidObjectFormat, message)
				return
			}
			tunnel.OnInput(envelope.SessionID, envelope.Payload.(string))
		case typeStart:
			tunnel.OnStart(envelope.SessionID)
		case typeEnd:
			tunnel.OnEnd(envelope.SessionID)
		default:
			log.Printf("%s,\n%s", errInvalidObjectFormat, message)
		}
	}

	handleConnection(tunnel)
}

func handleConnection(tunnel *SocketTunnel) {
	if tunnel.reconnectWait < 32 {
		tunnel.reconnectWait *= 2
	}
	// Socket connection can generate panic sometimes
	// trying for a graceful reconnect
	defer func() {
		if err := recover(); err != nil {
			//log.Println(err)
			log.Printf("Tunnel is attempting to establish connection in %v seconds...", tunnel.reconnectWait)
			time.Sleep(time.Duration(tunnel.reconnectWait) * time.Second)
			handleConnection(tunnel)
		}
	}()

	tunnel.socket.Connect()
}

// StopTunnel closes the active websocket connection
func (tunnel *SocketTunnel) StopTunnel() {
	tunnel.socket.Close()
}

// Send will send data in JSON format
func (tunnel *SocketTunnel) Send(sessionID string, payload string) {
	if tunnel.socket.IsConnected {
		envelope := envelope{
			Type:      typeOutput,
			Payload:   payload,
			SessionID: sessionID,
		}
		json, _ := json.Marshal(envelope)
		tunnel.socket.SendText(string(json))
	} else {
		log.Println("Cannot end session, are you even connected?")
	}
}

// End is used to send an end-session message in JSON format
func (tunnel *SocketTunnel) End(sessionID string) {
	if tunnel.socket.IsConnected {
		envelope := envelope{
			Type:      typeEnd,
			Payload:   sessionID,
			SessionID: sessionID,
		}
		json, _ := json.Marshal(envelope)
		tunnel.socket.SendText(string(json))
	} else {
		log.Println("Cannot end session, are you even connected?")
	}
}
