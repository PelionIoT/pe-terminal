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
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/creack/pty"
	"go.uber.org/zap"
)

/**
 * Created by Aditya Awasthi on 04/02/2021.
 * @author github.com/adwardstark
 */

type Terminal struct {
	cmd    *exec.Cmd
	tty    *os.File
	logger *zap.Logger
	mutex  *sync.Mutex
}

// Returns a new instance of tty
func NewTerminal(command string, logger *zap.Logger, onData func(string), onClose func()) (Terminal, error) {
	tLogger := logger.With(zap.String("component", "terminal"))
	tLogger.Info("Starting new session.")

	cmd := exec.Command(command)
	tty, err := pty.Start(cmd)
	if err != nil {
		return Terminal{}, err
	}

	term := Terminal{
		tty:    tty,
		cmd:    cmd,
		logger: tLogger,
		mutex:  &sync.Mutex{},
	}
	// Spin-up watcher-service
	go func() {
		tLogger.Debug("Starting watcher-service")
		for {
			buffer := make([]byte, 1024) // In bytes [ buffer-size ]
			readLength, err := term.tty.Read(buffer)
			if err != nil {
				tLogger.Debug("Failed to read from terminal", zap.Error(err))
				term.Close()
				onClose()
				return
			}
			payload := string(buffer[:readLength])
			tLogger.Debug("Sending message burst", zap.Int("bytes", readLength))
			onData(payload)
		}
	}()
	return term, nil
}

// Writes to the tty
func (term *Terminal) Write(command string) error {
	term.mutex.Lock()
	defer term.mutex.Unlock()
	term.logger.Debug("Received command", zap.String("command", command))
	_, err := term.tty.Write([]byte(strings.Trim(command, "\x00")))
	return err
}

// Resizes the tty window
func (term *Terminal) Resize(width uint16, height uint16) error {
	term.mutex.Lock()
	defer term.mutex.Unlock()
	term.logger.Debug("Resizing terminal", zap.Uint16("width", width), zap.Uint16("height", height))
	termSize := pty.Winsize{Y: height, X: width} // X is width, Y is height
	err := pty.Setsize(term.tty, &termSize)
	return err
}

// Closes the tty session
func (term *Terminal) Close() error {
	term.logger.Info("Stopping terminal.")
	if err := term.cmd.Process.Kill(); err != nil {
		return err
	}
	if _, err := term.cmd.Process.Wait(); err != nil {
		return err
	}
	if err := term.tty.Close(); err != nil {
		return err
	}
	term.logger.Debug("Terminal stopped successfully")
	return nil
}
