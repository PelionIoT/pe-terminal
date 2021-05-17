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

	flag.StringVar(&host, "host", "127.0.0.1", "Host address of terminal service")
	flag.StringVar(&port, "port", "3000", "Port number of terminal service")
	flag.StringVar(&endpoint, "endpoint", "/", "Endpoint to access terminal service")

	flag.Parse()

	log.Println("=====[ Pelion Edge Terminal ]=====")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	sessionsMap := make(map[string]*components.Terminal)
	commandsBufferMap := make(map[string]*bytes.Buffer)
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
			if _, ok := sessionsMap[sessionID]; ok {
				if sessionsMap[sessionID].IsPromptReady() {
					tunnel.Send(sessionID, components.EnterWithNewLine)
				} else {
					sessionsMap[sessionID].SetPromptReady()
				}
				tunnel.Send(sessionID, output)
			}
		}
		term.OnError = func(err error) {
			log.Printf("->onError() %v", err)
		}
		term.OnClose = func() {
			delete(sessionsMap, sessionID)
			delete(commandsBufferMap, sessionID)
			log.Printf("Terminal %s exited. Notifying cloud that this session is terminated.\n", sessionID)
			tunnel.End(sessionID)
		}

		sessionsMap[sessionID] = &term
		commandsBufferMap[sessionID] = &bytes.Buffer{}
		log.Printf("New session. Terminal %s created.\n", sessionID)
		sessionsMap[sessionID].InitPrompt()
	}
	tunnel.OnEnd = func(sessionID string) {
		if _, ok := sessionsMap[sessionID]; ok {
			log.Printf("Session ended. Killing terminal %s\n", sessionID)
			sessionsMap[sessionID].Write(string(components.ExitSession + components.Enter))
		}
	}
	tunnel.OnInput = func(sessionID string, payload string) {
		if _, ok := sessionsMap[sessionID]; ok {
			if payload != components.Enter {
				// Remove [ '\r' ] ENTER from payload while sending back
				tunnel.Send(sessionID, strings.Replace(payload, components.Enter, components.Empty, -1))
			}
			log.Printf("Received payload: %q from %s\n", payload, sessionID)
			commandsBufferMap[sessionID].WriteString(payload)
			if strings.Contains(payload, components.Enter) { // Execute on ENTER
				fullCommand := commandsBufferMap[sessionID].String()
				if strings.Contains(fullCommand, components.IsClearScreen) {
					log.Println("Clearing terminal screen")
					tunnel.Send(sessionID, components.ClearScreen)
					sessionsMap[sessionID].Write(components.Enter)
					commandsBufferMap[sessionID].Reset()
				} else if strings.Contains(fullCommand, components.ExitSession) {
					log.Println("Ending session, Killing terminal.")
					sessionsMap[sessionID].Write(fullCommand)
				} else {
					sessionsMap[sessionID].Write(fullCommand)
					if fullCommand != components.Enter { // To re-print the shell-prompt after command execution
						sessionsMap[sessionID].Write(components.Enter)
					}
					commandsBufferMap[sessionID].Reset()
				}
			} else if strings.Contains(payload, components.IsBackspaceKey) { // Execute on BACKSPACE
				log.Println("Backspace is pressed")
				tunnel.Send(sessionID, components.Backspace)
			} else if strings.Contains(payload, components.CtrlC) { // Execute on Control + C
				log.Println("Ctrl+C is pressed")
				sessionsMap[sessionID].Write(components.CtrlC + components.Enter)
			} else if strings.Contains(payload, components.CtrlX) { // Execute on Control + X
				log.Println("Ctrl+X is pressed")
				sessionsMap[sessionID].Write(components.CtrlX + components.Enter)
			} else if strings.Contains(payload, components.CtrlZ) { // Execute on Control + Z
				log.Println("Ctrl+Z is pressed")
				sessionsMap[sessionID].Write(components.CtrlZ + components.Enter)
			} else if strings.Contains(payload, components.EscKey) { // Execute on ESC
				log.Println("Esc is pressed")
				sessionsMap[sessionID].Write(components.EscKey + components.Enter)
			}
		}
	}
	tunnel.OnResize = func(sessionID string, width uint16, height uint16) {
		if _, ok := sessionsMap[sessionID]; ok {
			log.Printf("Resize terminal w: %v, h: %v from %s\n", width, height, sessionID)
			sessionsMap[sessionID].Resize(width, height)
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
