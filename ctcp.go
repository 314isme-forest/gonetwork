package gonetwork

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"log"
	"net"
	"sync"
)

type CTcp struct {
	config      ClientConfig
	connection  net.Conn
	stopChannel chan struct{}
	ctx         context.Context
	cancel      context.CancelFunc
	once        sync.Once
}

func TCPNewC(config ClientConfig, ctx context.Context, cancel context.CancelFunc) *CTcp {
	return &CTcp{
		config:      config,
		stopChannel: make(chan struct{}),
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (c *CTcp) Connect() error {
	var conn net.Conn
	var err error

	address := c.config.Address + ":" + c.config.Port

	if c.config.Secure {
		tlsConfig := &tls.Config{InsecureSkipVerify: true}
		conn, err = tls.Dial("tcp", address, tlsConfig)
	} else {
		conn, err = net.Dial("tcp", address)
	}
	if err != nil {
		return err
	}
	c.connection = conn

	if c.config.OnConnected != nil {
		c.config.OnConnected()
	}

	go c.readLoop()
	return nil
}

func (c *CTcp) readLoop() {
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
			var length uint32
			if err := binary.Read(c.connection, binary.BigEndian, &length); err != nil {
				log.Printf("Failed to read data length: %v", err)
				return
			}
			data := make([]byte, length)
			if _, err := c.connection.Read(data); err != nil {
				log.Printf("Failed to read data: %v", err)
				return
			}
			if c.config.OnData != nil {
				c.config.OnData(data)
			}
		}
	}
}

func (c *CTcp) Send(data []byte) error {
	length := uint32(len(data))
	if err := binary.Write(c.connection, binary.BigEndian, length); err != nil {
		log.Printf("Failed to write data length: %v", err)
		return err
	}
	if _, err := c.connection.Write(data); err != nil {
		log.Printf("Failed to write data: %v", err)
		return err
	}
	return nil
}

func (c *CTcp) Close() error {
	var closeErr error
	c.once.Do(func() {
		close(c.stopChannel)
		if c.connection != nil {
			closeErr = c.connection.Close()
			c.connection = nil
		}
	})
	return closeErr
}

func (c *CTcp) Disconnect() {
	c.cancel()
	if c.connection != nil {
		c.Close()
	}
}
