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

package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/PelionIoT/pe-terminal/components"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

/**
 * Created by Aditya Awasthi on 04/02/2021.
 * @author github.com/adwardstark
 */

// Config struct holds JSON-based configuration items
type Config struct {
	CloudURL *string `json:"cloud"`
	Command  *string `json:"command"`
	LogLevel *string `json:"logLevel"`
}

var logger *zap.Logger

func main() {
	var configFile string
	var config Config

	flag.StringVar(&configFile, "config", "", "Run with a JSON config")
	flag.Parse()

	// Set up logging
	atom := zap.NewAtomicLevel()
	logger = zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.Lock(os.Stdout),
		atom,
	))
	defer logger.Sync() // Flush buffer before closing

	logger.Info("=====[ Pelion Edge Terminal ]=====")

	// Parse configuration
	config = readConfig(configFile)
	atom.SetLevel(zapLogLevel(*config.LogLevel))

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// Setup tunnel-connection
	tunnel := components.NewTunnel(*config.CloudURL, *config.Command, logger)
	// Watch for interrupt
	go func() {
		<-interrupt
		tunnel.Close()
		<-time.After(time.Second)
		logger.Info("External interrupt, exiting pe-terminal.")
		os.Exit(1)
	}()

	// Start tunnel-connection
	tunnel.Connect()

	for {
		logger.Error("Tunnel disconnected. Attempting to establish connection")
		tunnel.HandleReConnection()
	}
}

func readConfig(fileName string) Config {
	if fileName != "" {
		logger.Info("Using config-file", zap.String("filename", fileName))
	} else {
		logger.Error("No config-file provided, use flag -config=<filename>.json")
		os.Exit(1)
	}

	configFile, err := os.Open(fileName)
	if err != nil {
		logger.Error("Failed to open config-file", zap.Error(err))
		os.Exit(1)
	}
	defer configFile.Close()

	buffer, err := ioutil.ReadAll(configFile)
	if err != nil {
		logger.Error("Failed to read config-file", zap.Error(err))
		os.Exit(1)
	}

	var config Config
	if err := json.Unmarshal([]byte(buffer), &config); err != nil {
		logger.Error("Failed to parse config-file", zap.Error(err))
		os.Exit(1)
	}

	// Check cloud-url
	if config.CloudURL != nil && *config.CloudURL != "" && !strings.HasPrefix(*config.CloudURL, "ws") {
		logger.Error("Invalid field `cloud` in config, should start with ws://")
		os.Exit(1)
	} else if config.CloudURL == nil {
		logger.Error("Missing field 'cloud` in config")
		os.Exit(1)
	}
	// Set logging-level [ defaults to: INFO]
	if config.LogLevel == nil && *config.LogLevel == "" {
		*config.LogLevel = "info"
	}
	// Set shell-command
	if config.Command == nil && *config.Command == "" {
		*config.Command = "/bin/bash" // [ default ]
	}

	return config
}

func zapLogLevel(logLevel string) zapcore.Level {
	switch strings.ToLower(logLevel) {
	case "debug":
		return zap.DebugLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	case "fatal":
		return zap.FatalLevel
	default:
		return zap.InfoLevel
	}
}
