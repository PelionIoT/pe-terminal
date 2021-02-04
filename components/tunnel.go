package components

import (
	"encoding/json"
	"log"
	"time"

	"github.com/sacOO7/gowebsocket"
)

/**
 * Created by Aditya Awasthi on 04/02/2021.
 * @author github.com/adwardstark
 */

// Base-level structure of a JSON message
// travelling through the tunnel
type messageEnvelope struct {
	Type      string `json:"type"`
	Payload   string `json:"payload"`
	SessionID string `json:"sessionID"`
}

// Envelope-types which will be
// used to parse the respective
// messageEnvelope
const (
	typeInput  = "input"
	typeOutput = "output"
	typeResize = "resize"
	typeStart  = "start"
	typeEnd    = "end"
)

// Checks if the given string
// is in a valid JSON format
func isValidJSON(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

// Converts the JSON string to messageEnvelope
func convertToEnvelope(data string) (messageEnvelope, error) {
	message := messageEnvelope{}
	err := json.Unmarshal([]byte(data), &message)
	return message, err
}

// Converts the messageEnvelope to JSON formatted string
func convertToJSON(envelope messageEnvelope) (string, error) {
	json, err := json.Marshal(envelope)
	return string(json), err
}

// SocketTunnel defines the base-level
// structure of the tunnel and all the
// available callbacks
type SocketTunnel struct {
	socket        gowebsocket.Socket
	reconnectWait int
	OnError       func(err error)
	OnStart       func(sessionID string)
	OnEnd         func(sessionID string)
	OnInput       func(sessionID string, payload string)
	OnResize      func(sessionID string, payload string)
}

// NewTunnel returns a new instance of
// SocketTunnel with a web-socket
// initialized with a connection URL
func NewTunnel(url string) SocketTunnel {
	return SocketTunnel{
		socket:        gowebsocket.New(url),
		reconnectWait: 0,
	}
}

// StartTunnel setups the necessary callbacks
// to monitor and results between different states
// and starts the connection through websocket
func (tunnel *SocketTunnel) StartTunnel() {
	// Setup options
	tunnel.socket.ConnectionOptions = gowebsocket.ConnectionOptions{
		UseSSL:         false, // Don't use SSL
		UseCompression: false, // Don't use compression
	}
	// Setup callback listeners for the socket connection
	// not all of them are available for external access
	// only few of them are exposed, rest are handled internally
	tunnel.socket.OnConnected = func(socket gowebsocket.Socket) {
		log.Printf("Tunnel connected at: %s\n", socket.Url)
		tunnel.reconnectWait = 0
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
			log.Printf("Ignoring message. Data could not be parsed as JSON,\n%s", message)
			return
		}
		// Convert message to MessageEnvelope
		if parsedMessageEnvelope, err := convertToEnvelope(message); err != nil {
			log.Printf("Ignoring message. Object format invalid,\n%s", message)
		} else {
			switch parsedMessageEnvelope.Type {
			case typeInput:
				tunnel.OnInput(parsedMessageEnvelope.SessionID, parsedMessageEnvelope.Payload)
			case typeResize:
				tunnel.OnResize(parsedMessageEnvelope.SessionID, parsedMessageEnvelope.Payload)
			case typeStart:
				tunnel.OnStart(parsedMessageEnvelope.SessionID)
			case typeEnd:
				tunnel.OnEnd(parsedMessageEnvelope.SessionID)
			default:
				log.Printf("Ignoring message. Object type invalid,\n%s", message)
			}
		}
	}

	handleConnection(tunnel)
}

func handleConnection(tunnel *SocketTunnel) {
	// Setup the reconnect timeout
	if tunnel.reconnectWait == 0 {
		tunnel.reconnectWait = 1
	} else if tunnel.reconnectWait < 32 {
		tunnel.reconnectWait *= 2
	}
	// Socket connection can generate panic sometimes
	// trying for a graceful reconnect
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
			log.Printf("Tunnel is attempting to establish connection in %v seconds...", tunnel.reconnectWait)
			time.Sleep(time.Duration(tunnel.reconnectWait) * time.Second)
			handleConnection(tunnel)
		}
	}()
	// Connect to socket
	tunnel.socket.Connect()
}

// StopTunnel closes the active websocket connection
func (tunnel *SocketTunnel) StopTunnel() {
	tunnel.socket.Close()
}

// Send will be used to send data in JSON format over websocket
func (tunnel *SocketTunnel) Send(sessionID string, payload string) {
	if tunnel.socket.IsConnected {
		messageEnvelope := messageEnvelope{
			Type:      typeOutput,
			Payload:   payload,
			SessionID: sessionID,
		}
		if jsonPayload, err := convertToJSON(messageEnvelope); err == nil {
			tunnel.socket.SendText(jsonPayload)
		} else {
			log.Println("Unable to convert message to JSON")
		}
	} else {
		log.Println("Cannot end session, are you even connected?")
	}
}

// End is used to send an end-session message
// in JSON format over the websocket
func (tunnel *SocketTunnel) End(sessionID string) {
	if tunnel.socket.IsConnected {
		messageEnvelope := messageEnvelope{
			Type:      typeEnd,
			Payload:   sessionID,
			SessionID: sessionID,
		}
		if jsonPayload, err := convertToJSON(messageEnvelope); err == nil {
			tunnel.socket.SendText(jsonPayload)
		} else {
			log.Println("Unable to convert message to JSON")
		}
	} else {
		log.Println("Cannot end session, are you even connected?")
	}
}
