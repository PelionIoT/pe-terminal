package main

import (
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/PelionIoT/pe-terminal/components"
)

/**
 * Created by Aditya Awasthi on 04/02/2021.
 * @author github.com/adwardstark
 */

func main() {
	var host string
	var port string
	var endpoint string

	flag.StringVar(&host, "host", "127.0.0.1", "Host address of terminal service")
	flag.StringVar(&port, "port", "3000", "Port number of terminal service")
	flag.StringVar(&endpoint, "endpoint", "/", "Endpoint to access terminal service")

	flag.Parse()

	log.Println("=====[ Pelion Edge Terminal ]=====")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	sessionsMap := make(map[string]*components.Terminal)
	tunnelURL := "ws://" + string(host+":"+port+endpoint)

	// Setup tunnel-connection
	tunnel := components.NewTunnel(tunnelURL)
	// Register callbacks to tunnel
	tunnel.OnStart = func(sessionID string) {
		term, err := components.NewTerminal("/bin/bash") // spawn new bash shell
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
