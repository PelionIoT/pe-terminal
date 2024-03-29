/*
Copyright 2021 Pelion Ltd.
Copyright (c) 2023 Izuma Networks


SPDX-License-Identifier: Apache-2.0

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package components

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
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
	socket        Socket
	reconnectWait int
	logger        *zap.Logger
	command       string
	mutex         *sync.Mutex
	sessionsMap   map[string]*Terminal
}

// NewTunnel returns a new instance of SocketTunnel
func NewTunnel(url string, command string, logger *zap.Logger) SocketTunnel {
	return SocketTunnel{
		socket: Socket{
			url:         url,
			logger:      logger.With(zap.String("component", "socket")),
			messageBus:  make(chan []byte),
			closeSignal: make(chan bool),
		},
		reconnectWait: 1,
		logger:        logger.With(zap.String("component", "tunnel")),
		command:       command,
		mutex:         &sync.Mutex{},
		sessionsMap:   make(map[string]*Terminal),
	}
}

// Connect the tunnel
func (tunnel *SocketTunnel) Connect() {
	tunnel.socket.SetupSocket(tunnel.onConnected, tunnel.onError, tunnel.onMessage)
}

// Close the tunnel
func (tunnel *SocketTunnel) Close() {
	tunnel.socket.Close()
}

func (tunnel *SocketTunnel) onConnected() {
	tunnel.logger.Info("Tunnel connected", zap.String("url", tunnel.socket.getURL()))
	tunnel.reconnectWait = 1
}

func (tunnel *SocketTunnel) onError(err error) {
	tunnel.logger.Error("Tunnel error", zap.Error(err))
	if !tunnel.socket.IsExited() {
		tunnel.HandleReConnection()
	}
}

func (tunnel *SocketTunnel) onMessage(message string) {
	if ok := isValidJSON(message); !ok {
		tunnel.logger.Error(errInvalidEnvelope, zap.String("payload", message))
		return
	}

	var envelope envelope
	decoder := json.NewDecoder(strings.NewReader(message))
	decoder.UseNumber()

	// Disallow unknown fields to validate data
	decoder.DisallowUnknownFields()

	err := decoder.Decode(&envelope)
	if err != nil {
		tunnel.logger.Error(errInvalidObjectFormat, zap.String("payload", message))
		return
	}

	switch envelope.Type {
	case typeResize:
		resize, ok := envelope.Payload.(map[string]interface{})
		if !ok {
			tunnel.logger.Error(errInvalidObjectFormat, zap.String("payload", message))
			return
		}

		width, widthOK := resize["width"]
		height, heightOK := resize["height"]
		if !widthOK || !heightOK {
			tunnel.logger.Error(errInvalidObjectFormat, zap.String("payload", message))
			return
		}

		w, ok := width.(json.Number)
		if !ok {
			tunnel.logger.Error(errInvalidObjectFormat, zap.String("payload", message))
			return
		}

		intWidth, err := w.Int64()
		if err != nil || intWidth < 0 {
			tunnel.logger.Error(errInvalidObjectFormat, zap.String("payload", message))
			return
		}

		h, ok := height.(json.Number)
		if !ok {
			tunnel.logger.Error(errInvalidObjectFormat, zap.String("payload", message))
			return
		}

		intHeight, err := h.Int64()
		if err != nil || intHeight < 0 {
			tunnel.logger.Error(errInvalidObjectFormat, zap.String("payload", message))
			return
		}

		tunnel.onResize(envelope.SessionID, intWidth, intHeight)
	case typeInput:
		// Validate payload type
		if reflect.TypeOf(envelope.Payload) == nil || reflect.TypeOf(envelope.Payload).Name() != "string" {
			tunnel.logger.Error(errInvalidObjectFormat, zap.String("payload", message))
			return
		}
		tunnel.onInput(envelope.SessionID, envelope.Payload.(string))
	case typeStart:
		tunnel.onStart(envelope.SessionID)
	case typeEnd:
		tunnel.onEnd(envelope.SessionID)
	default:
		tunnel.logger.Error(errInvalidObjectFormat, zap.String("payload", message))
	}
}

func (tunnel *SocketTunnel) onStart(sessionID string) {
	// Spawn a new shell
	term, err := NewTerminal(tunnel.command, tunnel.logger,
		func(output string) { // onData
			if tunnel.hasSession(sessionID) {
				tunnel.send(sessionID, output)
				tunnel.logger.Debug("Received response from terminal", zap.String("output", output), zap.String("sessionID", sessionID))
			}
		}, func() { // onClose
			tunnel.clearSession(sessionID)
			tunnel.logger.Info("Terminal exited, notifying cloud.", zap.String("sessionID", sessionID))
			tunnel.end(sessionID)
		})
	if err != nil {
		tunnel.logger.Error("Failed to initialize terminal", zap.Error(err))
		return
	}
	tunnel.setSession(sessionID, &term)
	tunnel.logger.Info("New session, terminal created.", zap.String("sessionID", sessionID))
}

func (tunnel *SocketTunnel) onEnd(sessionID string) {
	if tunnel.hasSession(sessionID) {
		tunnel.logger.Info("Session ended, killing terminal.", zap.String("sessionID", sessionID))
		err := tunnel.getSession(sessionID).Close()
		if err != nil {
			tunnel.logger.Error("Failed to kill terminal", zap.Error(err))
		}
	}
}

func (tunnel *SocketTunnel) onInput(sessionID string, payload string) {
	if tunnel.hasSession(sessionID) {
		err := tunnel.getSession(sessionID).Write(payload)
		if err != nil {
			tunnel.logger.Error("Failed to write on terminal", zap.Error(err))
		}
	}
}

func (tunnel *SocketTunnel) onResize(sessionID string, width int64, height int64) {
	if tunnel.hasSession(sessionID) {
		tunnel.logger.Info("Resize terminal", zap.String("sessionID", sessionID), zap.Int64("width", width), zap.Int64("height", height))
		err := tunnel.getSession(sessionID).Resize(uint16(width), uint16(height))
		if err != nil {
			tunnel.logger.Error("Failed to resize terminal", zap.Error(err))
		}
	}
}

// HandleReConnection re-establishes the connection after an issue after a delay (tunnel.reconnectWait)
func (tunnel *SocketTunnel) HandleReConnection() {
	tunnel.logger.Error("Tunnel is attempting to establish connection in " + fmt.Sprint(tunnel.reconnectWait) + " seconds...")
	time.Sleep(time.Duration(tunnel.reconnectWait) * time.Second)

	if tunnel.reconnectWait < 32 {
		tunnel.reconnectWait *= 2
	}
	tunnel.Connect()
}

func (tunnel *SocketTunnel) hasSession(sessionID string) bool {
	tunnel.mutex.Lock()
	defer tunnel.mutex.Unlock()
	_, ok := tunnel.sessionsMap[sessionID]
	return ok
}

func (tunnel *SocketTunnel) getSession(sessionID string) *Terminal {
	tunnel.mutex.Lock()
	defer tunnel.mutex.Unlock()
	session := tunnel.sessionsMap[sessionID]
	return session
}

func (tunnel *SocketTunnel) setSession(sessionID string, terminal *Terminal) {
	tunnel.mutex.Lock()
	defer tunnel.mutex.Unlock()
	tunnel.sessionsMap[sessionID] = terminal
}

func (tunnel *SocketTunnel) clearSession(sessionID string) {
	tunnel.mutex.Lock()
	defer tunnel.mutex.Unlock()
	delete(tunnel.sessionsMap, sessionID)
}

// Send will send data in JSON format
func (tunnel *SocketTunnel) send(sessionID string, payload string) {
	envelope := envelope{
		Type:      typeOutput,
		Payload:   payload,
		SessionID: sessionID,
	}
	json, _ := json.Marshal(envelope)
	tunnel.socket.Send(json)
}

// End is used to send an end-session message in JSON format
func (tunnel *SocketTunnel) end(sessionID string) {
	envelope := envelope{
		Type:      typeEnd,
		Payload:   sessionID,
		SessionID: sessionID,
	}
	json, _ := json.Marshal(envelope)
	tunnel.socket.Send(json)
}
