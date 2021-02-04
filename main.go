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

	flag.StringVar(&host, "host", "127.0.0.1", "Host address of terminal service")
	flag.StringVar(&port, "port", "3000", "Port number of terminal service")
	flag.Parse()

	log.Println("=====[ Pelion Edge Terminal ]=====")

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	sessionsMap := make(map[string]*components.Terminal)
	tunnelURL := "ws://" + string(host+":"+port) + "/terminal"

	// Setup tunnel-connection
	tunnel := components.NewTunnel(tunnelURL)
	// Register callbacks to tunnel
	tunnel.OnStart = func(sessionID string) {
		term, err := components.NewTerminal() // spawn new bash shell
		if err != nil {
			log.Println(err)
		}
		term.SetReadTimeout(2)
		term.OnData = func(output string) {
			log.Printf("->onData() %s\n", output)
			tunnel.Send(sessionID, output)
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
			log.Printf("Received command: %s from %s\n", payload, sessionID)
			sessionsMap[sessionID].Write(payload)
		}
	}
	tunnel.OnResize = func(sessionID string, payload string) {
		log.Printf("->onResize() sessionID: %s, payload: %s\n", sessionID, payload)
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
