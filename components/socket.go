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
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

/**
 * Created by Aditya Awasthi on 24/05/2021.
 * @author github.com/adwardstark
 */

type Socket struct {
	logger      *zap.Logger
	url         string
	messageBus  chan []byte
	closeSignal chan bool
	isExited    bool
}

func (socket *Socket) SetupSocket(onConnected func(), onError func(error), onMessage func(string)) {
	socket.isExited = false
	websocketDialer := &websocket.Dialer{}
	websocketDialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: false}
	connection, resp, err := websocketDialer.Dial(socket.url, nil)
	if err != nil {
		socket.logger.Debug("Websocket: Failed to connect", zap.Error(err))
		onError(err)
		return
	}
	if resp != nil {
		socket.logger.Debug("Websocket: Got an HTTP Response", zap.Int("code", resp.StatusCode), zap.String("status", resp.Status))
	}
	defer connection.Close()
	socket.logger.Debug("Websocket: Connected")
	onConnected()

	defaultCloseHandler := connection.CloseHandler()
	connection.SetCloseHandler(func(code int, text string) error {
		err := defaultCloseHandler(code, text)
		socket.logger.Debug("Websocket: Disconnected", zap.Error(err))
		onError(errors.New(text))
		return err
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, err := connection.ReadMessage()
			if err != nil {
				socket.logger.Debug("Websocket: Read-failed", zap.Error(err))
				onError(err)
				return
			}
			socket.logger.Debug("Websocket: Data-received", zap.ByteString("message", message))
			onMessage(string(message))
		}
	}()

	for {
		select {
		case message := <-socket.messageBus:
			err := connection.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				socket.logger.Debug("Websocket: Write-failed", zap.Error(err))
				return
			}
		case <-socket.closeSignal:
			socket.logger.Debug("Websocket: Closing connection")
			err := connection.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				socket.logger.Debug("Websocket: Write-close", zap.Error(err))
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}

func (socket *Socket) getURL() string {
	return socket.url
}

func (socket *Socket) IsExited() bool {
	return socket.isExited
}

func (socket *Socket) Send(message []byte) {
	socket.messageBus <- message
}

func (socket *Socket) Close() {
	socket.isExited = true
	socket.closeSignal <- true
}
