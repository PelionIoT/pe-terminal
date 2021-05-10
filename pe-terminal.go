package main

import (
	"bytes"
	"flag"
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

func main() {
	var host string
	var port string
	var endpoint string
	var commandBuffer bytes.Buffer

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
		term, err := components.NewTerminal() // spawn new bash shell
		if err != nil {
			log.Println(err)
		}
		term.SetReadTimeout(1)
		term.OnData = func(output string) {
			log.Printf("->onData() %s\n", output)
			tunnel.Send(sessionID, string(components.EnterWithNewLine+output))
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
		log.Printf("New session. Terminal %s created.\n", sessionID)
	}
	tunnel.OnEnd = func(sessionID string) {
		if _, ok := sessionsMap[sessionID]; ok {
			log.Printf("Session ended. Killing terminal %s\n", sessionID)
			sessionsMap[sessionID].Write("exit")
		}
	}
	tunnel.OnInput = func(sessionID string, payload string) {
		if _, ok := sessionsMap[sessionID]; ok {
			tunnel.Send(sessionID, payload)
			log.Printf("Received payload: %q from %s\n", payload, sessionID)
			commandBuffer.WriteString(payload)
			if strings.Contains(payload, components.Enter) { // Execute on ENTER
				fullCommand := commandBuffer.String()
				log.Printf("Completed command: %s\n", fullCommand)
				if strings.Contains(fullCommand, components.IsClearScreen) {
					log.Println("Clearing terminal screen")
					tunnel.Send(sessionID, components.ClearScreen)
				} else {
					sessionsMap[sessionID].Write(fullCommand)
				}
				sessionsMap[sessionID].Write(components.Enter)
				commandBuffer.Reset()
			} else if strings.Contains(payload, components.IsBackspaceKey) { // Execute on BACKSPACE
				log.Println("Backspace is pressed")
				tunnel.Send(sessionID, components.Backspace)
			}
		}
	}
	tunnel.OnResize = func(sessionID string, width uint16, height uint16) {
		if _, ok := sessionsMap[sessionID]; ok {
			log.Printf("Resize terminal w: %v, h: %v from %s\n", width, height, sessionID)
			sessionsMap[sessionID].Resize(width, height)
			sessionsMap[sessionID].Write(components.Enter)
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
