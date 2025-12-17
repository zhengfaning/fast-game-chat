package backend

import (
	"fmt"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
)

type MessageHandler func(data []byte)

type BackendPool struct {
	host     string
	port     int
	conns    chan *websocket.Conn
	maxConns int
	mu       sync.Mutex
    handler  MessageHandler
}

func NewBackendPool(host string, port int, poolSize int, handler MessageHandler) *BackendPool {
	return &BackendPool{
		host:     host,
		port:     port,
		conns:    make(chan *websocket.Conn, poolSize),
		maxConns: poolSize,
        handler:  handler,
	}
}

func (p *BackendPool) Get() (*websocket.Conn, error) {
	select {
	case conn := <-p.conns:
		return conn, nil
	default:
		// Pool empty, create new connection
		return p.createConnection()
	}
}

func (p *BackendPool) Put(conn *websocket.Conn) {
	select {
	case p.conns <- conn:
		// Returned to pool
	default:
		// Pool full, close connection
		conn.Close()
	}
}

func (p *BackendPool) createConnection() (*websocket.Conn, error) {
	u := url.URL{Scheme: "ws", Host: p.addr(), Path: "/"}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}

    // Start read loop
    go func() {
        defer conn.Close()
        for {
            _, message, err := conn.ReadMessage()
            if err != nil {
                // log.Printf("Backend read error: %v", err)
                return
            }
            if p.handler != nil {
                p.handler(message)
            }
        }
    }()

	return conn, nil
}

func (p *BackendPool) addr() string {
	// simple implementation
	return fmt.Sprintf("%s:%d", p.host, p.port)
}
