/*
Copyright 2021 Pelion Ltd.

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
	OnError       func(err error)
	OnStart       func(sessionID string)
	OnEnd         func(sessionID string)
	OnInput       func(sessionID string, payload string)
	OnResize      func(sessionID string, width int64, height int64)
}

// NewTunnel returns a new instance of SocketTunnel
func NewTunnel(url string, logger *zap.Logger) SocketTunnel {
	return SocketTunnel{
		socket: Socket{
			Url:           url,
			sendMutex:     &sync.Mutex{},
			receiveMutex:  &sync.Mutex{},
			logger:        logger,
		},
		reconnectWait: 1,
		logger:        logger,
	}
}

// StartTunnel will register callbacks and start connection
func (tunnel *SocketTunnel) StartTunnel() {
	tunnel.socket.OnConnected = func() {
		tunnel.logger.Info("Tunnel connected", zap.String("url", tunnel.socket.Url))
		tunnel.reconnectWait = 1
	}
	tunnel.socket.OnDisconnected = func(err error) {
		tunnel.logger.Info("Tunnel disconnected")
		tunnel.OnError(err)
		handleConnection(tunnel)
	}
	tunnel.socket.OnConnectError = func(err error) {
		tunnel.OnError(err)
	}
	tunnel.socket.OnTextMessage = func(message string) {
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

			tunnel.OnResize(envelope.SessionID, intWidth, intHeight)
		case typeInput:
			// Validate payload type
			if reflect.TypeOf(envelope.Payload) == nil || reflect.TypeOf(envelope.Payload).Name() != "string" {
				tunnel.logger.Error(errInvalidObjectFormat, zap.String("payload", message))
				return
			}
			tunnel.OnInput(envelope.SessionID, envelope.Payload.(string))
		case typeStart:
			tunnel.OnStart(envelope.SessionID)
		case typeEnd:
			tunnel.OnEnd(envelope.SessionID)
		default:
			tunnel.logger.Error(errInvalidObjectFormat, zap.String("payload", message))
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
			tunnel.logger.Error("Tunnel is attempting to establish connection in " + fmt.Sprint(tunnel.reconnectWait) + " seconds...")
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
		tunnel.logger.Error("Cannot access session, are you even connected?")
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
		tunnel.logger.Error("Cannot end session, are you even connected?")
	}
}
