package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"

	"github.com/PelionIoT/pe-terminal/components"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

/**
 * Created by Aditya Awasthi on 04/02/2021.
 * @author github.com/adwardstark
 */

type Config struct {
	CloudURL *string `json:"cloud"`
	Command  *string `json:"command"`
	LogLevel *string `json:"logLevel"`
}

var logger *zap.Logger

func main() {
	var configFile string
	var tunnelURL string

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

	command := "/bin/bash" // [ default ]
	if configFile != "" {
		logger.Info("Using config-file", zap.String("filename", configFile))
		config := readConfig(configFile)
		// Set cloud-url
		if config.CloudURL != nil && *config.CloudURL != "" {
			tunnelURL = makeWsURL(*config.CloudURL)
		} else {
			logger.Error("Missing field 'cloud` in config")
			os.Exit(1)
		}
		// Set logging-level [ defaults to: INFO]
		if config.LogLevel != nil && *config.LogLevel != "" {
			atom.SetLevel(zapLogLevel(*config.LogLevel))
		}
		// Set shell-command
		if config.Command != nil && *config.Command != "" {
			command = *config.Command
		}
	} else {
		logger.Error("No config-file provided, use flag -config=<filename>.json")
		os.Exit(1)
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	sessionsMap := make(map[string]*components.Terminal)

	// Setup tunnel-connection
	tunnel := components.NewTunnel(tunnelURL, logger)
	// Register callbacks to tunnel
	tunnel.OnStart = func(sessionID string) {
		term, err := components.NewTerminal(command, logger) // spawn new bash shell
		if err != nil {
			logger.Error("Error in initializing new terminal", zap.Error(err))
			return
		}
		term.OnData = func(output string) {
			if _, ok := sessionsMap[sessionID]; ok {
				tunnel.Send(sessionID, output)
				logger.Debug("Terminal Response", zap.String("output", output), zap.String("sessionID", sessionID))
			}
		}
		term.OnError = func(err error) {
			logger.Error("Terminal error", zap.Error(err))
		}
		term.OnClose = func() {
			delete(sessionsMap, sessionID)
			logger.Info("Terminal exited, notifying cloud.", zap.String("sessionID", sessionID))
			tunnel.End(sessionID)
		}

		sessionsMap[sessionID] = &term
		sessionsMap[sessionID].InitPrompt()
		logger.Info("New session, terminal created.", zap.String("sessionID", sessionID))
	}
	tunnel.OnEnd = func(sessionID string) {
		if _, ok := sessionsMap[sessionID]; ok {
			logger.Info("Session ended, killing terminal.", zap.String("sessionID", sessionID))
			sessionsMap[sessionID].Close()
		}
	}
	tunnel.OnInput = func(sessionID string, payload string) {
		if _, ok := sessionsMap[sessionID]; ok {
			sessionsMap[sessionID].Write(payload)
		}
	}
	tunnel.OnResize = func(sessionID string, width int64, height int64) {
		if _, ok := sessionsMap[sessionID]; ok {
			logger.Info("Resize terminal", zap.String("sessionID", sessionID), zap.Int64("width", width), zap.Int64("height", height))
			sessionsMap[sessionID].Resize(uint16(width), uint16(height))
		}
	}
	tunnel.OnError = func(err error) {
		logger.Error("Tunnel error", zap.Error(err))
	}
	// Start tunnel-connection
	tunnel.StartTunnel()
	// Wait for interrupt
	for {
		select {
		case <-interrupt:
			logger.Info("External interrupt, exiting pe-terminal.")
			// Stop tunnel-connection
			tunnel.StopTunnel()
			return
		}
	}
}

func readConfig(fileName string) Config {
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
	return config
}

func makeWsURL(url string) string {
	if strings.HasPrefix(url, "http") {
		url = strings.Replace(url, "http", "ws", -1)
	} else if strings.HasPrefix(url, "https") {
		url = strings.Replace(url, "https", "ws", -1) // Should be 'wss://', skipping for now as SSL is not supported.
	}
	return url
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
