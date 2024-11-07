package gonetwork

import (
	"context"
	"log"

	"google.golang.org/protobuf/proto"
)

type Client struct {
	config    ClientConfig
	tcpClient *CTcp
	wsClient  *CWs
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewClient(config ClientConfig) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	client := &Client{
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}
	switch config.Type {
	case "TCP":
		client.tcpClient = TCPNewC(config, ctx, cancel)
	case "WS":
		client.wsClient = WSNewC(config, ctx, cancel)
	default:
		cancel()
		return nil
	}
	return client
}

func (c *Client) Send(dataHandler string, dataType string, dataProto proto.Message) error {
	dataByte := Encode(dataHandler, dataType, dataProto)
	var err error
	if c.config.Type == "TCP" {
		err = c.tcpClient.Send(dataByte)
	} else if c.config.Type == "WS" {
		err = c.wsClient.Send(dataByte)
	}
	if err != nil {
		log.Printf("Send error: %v", err)
	}
	return err
}

func (c *Client) Connect() error {
	if c.config.Type == "TCP" {
		return c.tcpClient.Connect()
	} else if c.config.Type == "WS" {
		return c.wsClient.Connect()
	}
	return nil
}

func (c *Client) Disconnect() {
	if c.tcpClient != nil {
		c.tcpClient.Disconnect()
	}
	if c.wsClient != nil {
		c.wsClient.Disconnect()
	}
}
