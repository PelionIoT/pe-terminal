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
	"crypto/tls"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

/**
 * Created by Aditya Awasthi on 24/05/2021.
 * @author github.com/adwardstark
 */

type Socket struct {
	connection     *websocket.Conn
	requestHeader  http.Header
	useSSL         bool
	timeout        time.Duration
	sendMutex      *sync.Mutex
	receiveMutex   *sync.Mutex
	logger         *zap.Logger
	Url            string
	IsConnected    bool
	OnConnected    func(socket Socket)
	OnTextMessage  func(message string, socket Socket)
	OnConnectError func(err error, socket Socket)
	OnDisconnected func(err error, socket Socket)
}

func (socket *Socket) Connect() {
	var err error
	var resp *http.Response

	websocketDialer := &websocket.Dialer{}
	websocketDialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: socket.useSSL}

	socket.connection, resp, err = websocketDialer.Dial(socket.Url, socket.requestHeader)

	if err != nil {
		socket.logger.Debug("Websocket: Failed to connect", zap.Error(err))
		if resp != nil {
			socket.logger.Debug("Websocket: Got an HTTP Response", zap.Int("code", resp.StatusCode), zap.String("status", resp.Status))
		}
		socket.IsConnected = false
		if socket.OnConnectError != nil {
			socket.OnConnectError(err, *socket)
		}
		return
	}

	socket.logger.Debug("Websocket: Connected")

	if socket.OnConnected != nil {
		socket.IsConnected = true
		socket.OnConnected(*socket)
	}

	defaultCloseHandler := socket.connection.CloseHandler()
	socket.connection.SetCloseHandler(func(code int, text string) error {
		err := defaultCloseHandler(code, text)
		socket.logger.Debug("Websocket: Disconnected", zap.Error(err))
		if socket.OnDisconnected != nil {
			socket.IsConnected = false
			socket.OnDisconnected(errors.New(text), *socket)
		}
		return err
	})

	go func() {
		for {
			socket.receiveMutex.Lock()
			if socket.timeout != 0 {
				socket.connection.SetReadDeadline(time.Now().Add(socket.timeout))
			}
			messageType, message, err := socket.connection.ReadMessage()
			socket.receiveMutex.Unlock()
			if err != nil {
				socket.logger.Debug("Websocket: Read-failed", zap.Error(err))
				if socket.OnDisconnected != nil {
					socket.IsConnected = false
					socket.OnDisconnected(err, *socket)
				}
				return
			}
			socket.logger.Debug("Websocket: Data-received", zap.ByteString("message", message))

			if messageType == websocket.TextMessage && socket.OnTextMessage != nil {
				socket.OnTextMessage(string(message), *socket)
			} else {
				socket.logger.Debug("Websocket: Unsupported message-type")
			}
		}
	}()
}

func (socket *Socket) SendText(message string) {
	err := socket.send(websocket.TextMessage, []byte(message))
	if err != nil {
		socket.logger.Debug("Websocket: Write-failed", zap.Error(err))
		return
	}
}

func (socket *Socket) send(messageType int, data []byte) error {
	socket.sendMutex.Lock()
	err := socket.connection.WriteMessage(messageType, data)
	socket.sendMutex.Unlock()
	return err
}

func (socket *Socket) Close() {
	err := socket.send(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		socket.logger.Debug("Websocket: Write-close", zap.Error(err))
	}
	socket.connection.Close()
	if socket.OnDisconnected != nil {
		socket.IsConnected = false
		socket.OnDisconnected(err, *socket)
	}
}
