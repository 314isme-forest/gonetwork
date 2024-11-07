package gonetwork

import (
	"context"
	"log"
	"net/url"
	"sync"

	"github.com/coder/websocket"
)

type CWs struct {
	config      ClientConfig
	conn        *websocket.Conn
	stopChannel chan struct{}
	ctx         context.Context
	cancel      context.CancelFunc
	once        sync.Once
}

func WSNewC(config ClientConfig, ctx context.Context, cancel context.CancelFunc) *CWs {
	return &CWs{
		config:      config,
		stopChannel: make(chan struct{}),
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (c *CWs) Connect() error {
	ctx := context.Background()
	u := url.URL{Scheme: "wss", Host: c.config.Address + ":" + c.config.Port, Path: "/"}
	if !c.config.Secure {
		u.Scheme = "ws"
	}
	conn, _, err := websocket.Dial(ctx, u.String(), nil)
	if err != nil {
		log.Printf("Failed to connect to server: %v", err)
		return err
	}
	c.conn = conn

	if c.config.OnConnected != nil {
		c.config.OnConnected()
	}

	go c.readLoop()
	return nil
}

func (c *CWs) readLoop() {
	defer func() {
		if c.config.OnDisconnected != nil {
			c.config.OnDisconnected()
		}
		c.Close()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			_, data, err := c.conn.Read(c.ctx)
			if err != nil {
				log.Printf("Failed to read data: %v", err)
				return
			}
			if c.config.OnData != nil {
				c.config.OnData(data)
			}
		}
	}
}

func (c *CWs) Send(data []byte) error {
	if err := c.conn.Write(c.ctx, websocket.MessageBinary, data); err != nil {
		log.Printf("Failed to send data: %v", err)
		return err
	}
	return nil
}

func (c *CWs) Close() error {
	var closeErr error
	c.once.Do(func() {
		close(c.stopChannel)
		if c.conn != nil {
			closeErr = c.conn.Close(websocket.StatusNormalClosure, "")
			c.conn = nil
		}
	})
	return closeErr
}

func (c *CWs) Disconnect() {
	c.cancel()
	if c.conn != nil {
		c.Close()
	}
}
