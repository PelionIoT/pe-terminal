package components

import (
	"bufio"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/creack/pty"
)

/**
 * Created by Aditya Awasthi on 04/02/2021.
 * @author github.com/adwardstark
 */

// Terminal defines the base-level
// structure for the terminal and
// all the available callbacks
type Terminal struct {
	pty         *os.File
	readTimeout int64
	OnData      func(output string)
	OnError     func(err error)
	OnClose     func()
}

// This channel is used to send
// stream of data read from the
// pty file [ /dev/ptmx ]
var broadcast = make(chan string)

// NewTerminal will return a new instance of Terminal
// and attaches a pty to it, [ /dev/ptmx ] with a bash-shell
// and starts a watcher-service which monitors the pty
// and sends the data-stream via the broadcast whenever
// something is written to the pty
func NewTerminal() (Terminal, error) {
	log.Println("Starting new session")
	term := Terminal{
		readTimeout: 5, // defaults to 5 seconds
	}
	c := exec.Command("bash") // bash-shell
	pty, err := pty.Start(c)
	if err == nil {
		term.pty = pty
		go watchPty(term.pty) // watcher-service
	}
	return term, err
}

// Writes a command to the pty
func (term *Terminal) Write(command string) {
	log.Printf("->Write() command: %s", command)
	if _, err := term.pty.Write([]byte(string(command + "\r"))); err != nil {
		//log.Println(err)
		if term.OnError != nil {
			term.OnError(err)
		}
	} else {
		term.processResponse()
	}
}

// Watcher-service for the pty, monitors everything
// written to the pty and send a broadcast stream of data
func watchPty(file *os.File) {
	log.Println("Starting watcher-service")
	stdoutScanner := bufio.NewScanner(file)
	for stdoutScanner.Scan() {
		broadcast <- stdoutScanner.Text()
	}
}

// Reads data from received from the broadcast,
// for a specific interval defined at the time
// of initialization, blocks the write operation
// for read to complete before the next write
// to provide syncronization across read/writes
func (term *Terminal) processResponse() {
	timeout := term.readTimeout
	log.Printf("Reading from terminal for %v seconds\n", timeout)
	timeoutAfter := time.After(time.Duration(timeout) * time.Second)
	for {
		select {
		case data := <-broadcast:
			//log.Println("->onTerminal()", data)
			if term.OnData != nil {
				term.OnData(data)
			}
			if term.OnClose != nil {
				if data == "exit" {
					term.Close()
					term.OnClose()
				}
			}
		case <-timeoutAfter:
			log.Println("Read timeout, returning control")
			return
		}
	}
}

// SetReadTimeout can be used to configure the interval
// for the read to occurr from pty before next write operation
func (term *Terminal) SetReadTimeout(timeout int64) {
	term.readTimeout = timeout
}

// Resize is yet to be implemented
func (term *Terminal) Resize(width int, height int) {
	log.Printf("->Resize() width: %v, height: %v\n", width, height)
}

// Close is used to terminate the pty-session
func (term *Terminal) Close() error {
	// Make sure to close the pty at the end.
	err := term.pty.Close()
	return err
}
