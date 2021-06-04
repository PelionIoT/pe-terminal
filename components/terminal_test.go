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
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

/**
 * Created by Aditya Awasthi on 03/06/2021.
 * @author github.com/adwardstark
 */

var logger *zap.Logger
var isCompleted chan bool
var timeoutAfter <-chan time.Time
var runInScope = beforeEach(setup, teardown)

func beforeEach(setup func(), teardown func()) func(func()) {
	return func(testFunc func()) {
		setup()
		testFunc()
		teardown()
	}
}

func setup() {
	logger, _ = zap.NewProduction()
	isCompleted = make(chan bool)
	timeoutAfter = time.After(time.Duration(2) * time.Second)
}

func teardown() {
	defer logger.Sync() // flushes buffer, if any
}

func TestTerminalSetup(t *testing.T) {
	runInScope(func() {
		term, err := NewTerminal("/bin/bash", logger)
		defer term.Close() // gracefully close, best effort
		if err != nil {
			t.Fail()
		}
	})
}

func TestTerminalResize(t *testing.T) {
	runInScope(func() {
		go func() {
			term, _ := NewTerminal("/bin/bash", logger)
			defer term.Close() // gracefully close, best effort

			term.OnError = func(err error) {
				t.Fatal(err)
			}
			term.Resize(120, 50)
		}()

		for {
			select {
			case <-timeoutAfter:
				return
			}
		}
	})
}

func TestTerminalPromptReturned(t *testing.T) {
	runInScope(func() {
		go func() {
			term, _ := NewTerminal("/bin/bash", logger)
			defer term.Close() // gracefully close, best effort
			term.OnData = func(output string) {
				for strings.Contains(output, "bash") {
					isCompleted <- true
				} // If not found, test will fail on timeout
			}
			term.OnError = func(err error) {
				t.Fatal(err)
				isCompleted <- false
			}
			term.InitPrompt()
		}()

		for {
			select {
			case <-isCompleted:
				return
			case <-timeoutAfter:
				t.Fatal("Timeout, did not received response: \"bash\" within 2 seconds")
			}
		}
	})
}

func TestTerminalCommandExecuted(t *testing.T) {
	runInScope(func() {
		go func() {
			term, _ := NewTerminal("/bin/bash", logger)
			defer term.Close() // gracefully close, best effort
			term.OnData = func(output string) {
				for strings.Contains(output, "echo something") {
					isCompleted <- true
				}
			}
			term.OnError = func(err error) {
				t.Fatal(err)
				isCompleted <- false
			}
			term.Write("echo something\r")
		}()

		for {
			select {
			case <-isCompleted:
				return
			case <-timeoutAfter:
				t.Fatal("Timeout, did not received response: \"echo something\" within 2 seconds")
			}
		}
	})
}
