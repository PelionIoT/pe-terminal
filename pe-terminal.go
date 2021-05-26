package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/PelionIoT/pe-terminal/components"
)

/**
 * Created by Aditya Awasthi on 04/02/2021.
 * @author github.com/adwardstark
 */

type Config struct {
	CloudURL      *string `json:"cloud"`
	Command       *string `json:"command"`
	SkipTLSVerify *bool   `json:"noValidate"`
	SSLCert       *string `json:"certificate"`
	SSLKey        *string `json:"key"`
}

func main() {
	var configFile string
	var tunnelURL string

	flag.StringVar(&configFile, "config", "", "Run with a JSON config")
	flag.Parse()

	log.Println("=====[ Pelion Edge Terminal ]=====")

	command := "/bin/bash" // [ default ]
	if configFile != "" {
		log.Println("Using config-file:", configFile)
		config := readConfig(configFile)
		if config.CloudURL != nil && *config.CloudURL != "" {
			tunnelURL = makeWsURL(*config.CloudURL)
		} else {
			log.Println("Missing field 'cloud` in config")
			os.Exit(1)
		}
		if config.Command != nil && *config.Command != "" {
			command = *config.Command
		}
	} else {
		log.Println("No config-file provided, use flag -config=<filename>.json")
		os.Exit(1)
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	sessionsMap := make(map[string]*components.Terminal)

	// Setup tunnel-connection
	tunnel := components.NewTunnel(tunnelURL)
	// Register callbacks to tunnel
	tunnel.OnStart = func(sessionID string) {
		term, err := components.NewTerminal(command) // spawn new bash shell
		if err != nil {
			log.Println(err)
			return
		}
		term.OnData = func(output string) {
			log.Printf("->onData() %q\n", output)
			if _, ok := sessionsMap[sessionID]; ok {
				tunnel.Send(sessionID, output)
			}
		}
		term.OnError = func(err error) {
			log.Printf("->onError() %v", err)
		}
		term.OnClose = func() {
			delete(sessionsMap, sessionID)
			log.Printf("Terminal %s exited. Notifying cloud that this session is terminated.\n", sessionID)
			tunnel.End(sessionID)
		}

		sessionsMap[sessionID] = &term
		sessionsMap[sessionID].InitPrompt()
		log.Printf("New session. Terminal %s created.\n", sessionID)
	}
	tunnel.OnEnd = func(sessionID string) {
		if _, ok := sessionsMap[sessionID]; ok {
			log.Printf("Session ended. Killing terminal %s\n", sessionID)
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
			log.Printf("Resize terminal w: %v, h: %v from %s\n", width, height, sessionID)
			sessionsMap[sessionID].Resize(uint16(width), uint16(height))
		}
	}
	tunnel.OnError = func(err error) {
		log.Println("->onError() ", err)
	}
	// Start tunnel-connection
	tunnel.StartTunnel()
	// Wait for interrupt
	for {
		select {
		case <-interrupt:
			log.Println("->External-Interrupt, exiting.")
			// Stop tunnel-connection
			tunnel.StopTunnel()
			return
		}
	}
}

func readConfig(fileName string) Config {
	configFile, err := os.Open(fileName)
	if err != nil {
		log.Println("Error:", err)
		os.Exit(1)
	}
	defer configFile.Close()

	buffer, err := ioutil.ReadAll(configFile)
	if err != nil {
		log.Println("Error:", err)
		os.Exit(1)
	}

	var config Config
	if err := json.Unmarshal([]byte(buffer), &config); err != nil {
		log.Println("Error:", err)
		os.Exit(1)
	}
	return config
}

func makeWsURL(url string) string {
	if strings.HasPrefix(url, "http") {
		url = strings.Replace(url, "http", "ws", -1)
	} else if strings.HasPrefix(url, "https") {
		url = strings.Replace(url, "https", "ws", -1) // Should be 'wss://', skipping for now as SSL support is not implemented yet.
	}
	return url
}
