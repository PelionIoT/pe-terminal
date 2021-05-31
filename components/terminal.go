package components

import (
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/creack/pty"
	"go.uber.org/zap"
)

type Terminal struct {
	cmd           *exec.Cmd
	tty           *os.File
	ttyData       chan string
	ttyError      chan bool
	readTimeout   int64
	maxBufferSize int64
	logger        *zap.Logger
	OnData        func(output string)
	OnError       func(err error)
	OnClose       func()
}

// Returns a new instance of tty
func NewTerminal(command string, logger *zap.Logger) (Terminal, error) {
	logger.Info("Starting new session.")
	term := Terminal{
		ttyData:       make(chan string),
		ttyError:      make(chan bool),
		readTimeout:   100,  // In millisceonds [ default ]
		maxBufferSize: 1024, // In bytes [ default ]
		logger:        logger,
	}
	cmd := exec.Command(command)
	tty, err := pty.Start(cmd)
	if err == nil {
		term.tty = tty
		term.cmd = cmd
		go term.watchTTY() // Spin-up watcher-service
	}
	return term, err
}

// Read the initial prompt at setup
func (term *Terminal) InitPrompt() {
	term.watchComms(2000)
}

// Configure read timeout for the comms
func (term *Terminal) SetReadTimeout(timeout int64) {
	term.logger.Debug("Setting read-timeout", zap.Int64("timeout", timeout))
	term.readTimeout = timeout
}

// Configure size of buffer to read from tty
func (term *Terminal) SetTTYBufferSize(size int64) {
	term.logger.Debug("Setting TTY buffer-size", zap.Int64("size", size))
	term.maxBufferSize = size
}

// Monitors everything written to the tty and sends a respective broadcast
func (term *Terminal) watchTTY() {
	term.logger.Debug("Starting watcher-service")
	for {
		buffer := make([]byte, term.maxBufferSize)
		readLength, err := term.tty.Read(buffer)
		if err != nil {
			term.logger.Debug("Failed to read from terminal", zap.Error(err))
			term.ttyError <- true
			return
		}
		payload := string(buffer[:readLength])
		term.logger.Debug("Sending message burst", zap.Int("bytes", readLength))
		term.ttyData <- payload
	}
}

// Reads the broadcast coming through channels
func (term *Terminal) watchComms(timeout int64) {
	term.logger.Debug("Reading from terminal until timeout", zap.Int64("inMillis", timeout))
	timeoutAfter := time.After(time.Duration(timeout) * time.Millisecond)
	for {
		select {
		case data := <-term.ttyData:
			term.logger.Debug("Received data signal, forwarding the payload")
			if term.OnData != nil {
				term.OnData(data)
			}
		case <-term.ttyError:
			term.logger.Debug("Received error signal, attempting to close session")
			term.Close()
			if term.OnClose != nil {
				term.OnData("Logging out, bye :)\r\n")
				term.OnClose()
			}
		case <-timeoutAfter:
			term.logger.Debug("Read timeout, returning control")
			return
		}
	}
}

// Writes to the tty
func (term *Terminal) Write(command string) {
	term.logger.Debug("Execute command", zap.String("command", command))
	if _, err := term.tty.Write([]byte(strings.Trim(command, "\x00"))); err != nil {
		if term.OnError != nil {
			term.OnError(err)
		}
	} else {
		term.watchComms(term.readTimeout)
	}
}

// Resizes the tty window
func (term *Terminal) Resize(width uint16, height uint16) {
	term.logger.Debug("Resizing terminal", zap.Uint16("width", width), zap.Uint16("height", height))
	termSize := pty.Winsize{Y: height, X: width} // X is width, Y is height
	if err := pty.Setsize(term.tty, &termSize); err != nil {
		if term.OnError != nil {
			term.OnError(err)
		}
	}
}

// Closes the tty session
func (term *Terminal) Close() {
	term.logger.Info("Gracefully stopping terminal.")
	if err := term.cmd.Process.Kill(); err != nil {
		term.logger.Error("Failed to kill process", zap.Error(err))
	}
	if _, err := term.cmd.Process.Wait(); err != nil {
		term.logger.Error("Failed to wait for process to exit", zap.Error(err))
	}
	if err := term.tty.Close(); err != nil {
		term.logger.Error("Failed to close terminal gracefully", zap.Error(err))
	} else {
		term.logger.Debug("Terminal stopped successfully")
	}
}
