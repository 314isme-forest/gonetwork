package gonetwork

import (
	"crypto/tls"
	"errors"
	"log"
	"net"
	"sync"

	"google.golang.org/protobuf/proto"
)

type ServerConfig struct {
	Address        string
	Port           string
	Domain         string
	TCPPort        string
	WSPort         string
	Secure         bool
	MaxWorkers     int
	OnConnected    func(connectionID string)
	OnData         func(connectionID string, data []byte)
	OnDisconnected func(connectionID string)
}

type ClientConfig struct {
	Address        string
	Port           string
	Secure         bool
	Type           string
	OnConnected    func()
	OnData         func(data []byte)
	OnDisconnected func()
}

func TLSCert(domain string) (tls.Certificate, error) {
	return tls.LoadX509KeyPair("/etc/letsencrypt/live/"+domain+"/fullchain.pem", "/etc/letsencrypt/live/"+domain+"/privkey.pem")
}

type ConnPool struct {
	mu       sync.Mutex
	idle     []net.Conn
	active   int
	maxConns int
}

func NewConnPool(maxConns int) *ConnPool {
	return &ConnPool{maxConns: maxConns}
}

func (p *ConnPool) Get() (net.Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.idle) > 0 {
		conn := p.idle[len(p.idle)-1]
		p.idle = p.idle[:len(p.idle)-1]
		return conn, nil
	}
	if p.active < p.maxConns {
		conn, err := net.Dial("tcp", "server_address")
		if err != nil {
			return nil, err
		}
		p.active++
		return conn, nil
	}
	return nil, errors.New("no available connections")
}

func (p *ConnPool) Put(conn net.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.idle = append(p.idle, conn)
}

func (p *ConnPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, conn := range p.idle {
		conn.Close()
	}
	p.idle = nil
	p.active = 0
}

func Encode(dataHandler, dataType string, dataProto proto.Message) []byte {
	protoBytes, err := proto.Marshal(dataProto)
	if err != nil {
		log.Fatalf("Failed to marshal data: %v", err)
		return nil
	}
	networkData := &SMessage{
		Handler: dataHandler,
		Type:    dataType,
		Proto:   protoBytes,
	}
	networkDataBytes, err := proto.Marshal(networkData)
	if err != nil {
		log.Fatalf("Failed to marshal network data: %v", err)
		return nil
	}
	return networkDataBytes
}

func Decode(data []byte) (string, string, []byte) {
	var networkData SMessage
	if err := proto.Unmarshal(data, &networkData); err != nil {
		log.Fatalf("Failed to unmarshal network data: %v", err)
		return "", "", nil
	}
	dataHandler := networkData.Handler
	dataType := networkData.Type
	return dataHandler, dataType, networkData.Proto
}

func DecodeProto(data []byte, dataProto proto.Message) proto.Message {
	if err := proto.Unmarshal(data, dataProto); err != nil {
		log.Fatalf("Failed to unmarshal data: %v", err)
		return nil
	}
	return dataProto
}
