package components

import (
	"crypto/tls"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Socket struct {
	connection     *websocket.Conn
	requestHeader  http.Header
	useSSL         bool
	timeout        time.Duration
	sendMutex      *sync.Mutex
	receiveMutex   *sync.Mutex
	Url            string
	IsConnected    bool
	OnConnected    func(socket Socket)
	OnTextMessage  func(message string, socket Socket)
	OnConnectError func(err error, socket Socket)
	OnDisconnected func(err error, socket Socket)
}

func (socket *Socket) Connect() {
	var err error
	var resp *http.Response

	websocketDialer := &websocket.Dialer{}
	websocketDialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: socket.useSSL}

	socket.connection, resp, err = websocketDialer.Dial(socket.Url, socket.requestHeader)

	if err != nil {
		log.Println("Failed to connect: ", err)
		if resp != nil {
			log.Printf("HTTP Response %d status: %s", resp.StatusCode, resp.Status)
		}
		socket.IsConnected = false
		if socket.OnConnectError != nil {
			socket.OnConnectError(err, *socket)
		}
		return
	}

	log.Println("Connected Successfully!")

	if socket.OnConnected != nil {
		socket.IsConnected = true
		socket.OnConnected(*socket)
	}

	defaultCloseHandler := socket.connection.CloseHandler()
	socket.connection.SetCloseHandler(func(code int, text string) error {
		result := defaultCloseHandler(code, text)
		log.Println("Disconnected: ", result)
		if socket.OnDisconnected != nil {
			socket.IsConnected = false
			socket.OnDisconnected(errors.New(text), *socket)
		}
		return result
	})

	go func() {
		for {
			socket.receiveMutex.Lock()
			if socket.timeout != 0 {
				socket.connection.SetReadDeadline(time.Now().Add(socket.timeout))
			}
			messageType, message, err := socket.connection.ReadMessage()
			socket.receiveMutex.Unlock()
			if err != nil {
				//log.Println("read:", err)
				if socket.OnDisconnected != nil {
					socket.IsConnected = false
					socket.OnDisconnected(err, *socket)
				}
				return
			}
			//log.Println("recv: %s", message)

			if messageType == websocket.TextMessage && socket.OnTextMessage != nil {
				socket.OnTextMessage(string(message), *socket)
			}
		}
	}()
}

func (socket *Socket) SendText(message string) {
	err := socket.send(websocket.TextMessage, []byte(message))
	if err != nil {
		log.Println("write:", err)
		return
	}
}

func (socket *Socket) send(messageType int, data []byte) error {
	socket.sendMutex.Lock()
	err := socket.connection.WriteMessage(messageType, data)
	socket.sendMutex.Unlock()
	return err
}

func (socket *Socket) Close() {
	err := socket.send(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		log.Println("write close:", err)
	}
	socket.connection.Close()
	if socket.OnDisconnected != nil {
		socket.IsConnected = false
		socket.OnDisconnected(err, *socket)
	}
}
