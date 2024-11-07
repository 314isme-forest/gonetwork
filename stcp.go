package gonetwork

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"log"
	"net"
	"sync"
)

type STcp struct {
	config      ServerConfig
	tcpListener net.Listener
	connections sync.Map
	acceptorWG  sync.WaitGroup
	handlerWG   sync.WaitGroup
	taskChan    chan net.Conn
	connPool    *ConnPool
}

type TCPConnectionEntry struct {
	Conn net.Conn
}

func TCPNewS(config ServerConfig) *STcp {
	server := &STcp{
		config:   config,
		taskChan: make(chan net.Conn, config.MaxWorkers),
		connPool: NewConnPool(config.MaxWorkers),
	}
	server.startWorkers()
	return server
}

func (s *STcp) startWorkers() {
	for i := 0; i < s.config.MaxWorkers; i++ {
		s.handlerWG.Add(1)
		go s.worker()
	}
}

func (s *STcp) Listen(ctx context.Context) error {
	var err error
	if s.config.Secure {
		cert, err := TLSCert(s.config.Domain)
		if err != nil {
			return err
		}

		tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
		s.tcpListener, err = tls.Listen("tcp", s.config.Address+":"+s.config.TCPPort, tlsConfig)
		if err != nil {
			return err
		}
	} else {
		s.tcpListener, err = net.Listen("tcp", s.config.Address+":"+s.config.TCPPort)
	}
	if err != nil {
		return err
	}

	s.acceptorWG.Add(1)
	go s.acceptConnections(ctx)
	return nil
}

func (s *STcp) acceptConnections(ctx context.Context) {
	defer s.acceptorWG.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn, err := s.tcpListener.Accept()
			if err != nil {
				log.Printf("Failed to accept connection: %v", err)
				continue
			}
			s.taskChan <- conn
		}
	}
}

func (s *STcp) worker() {
	defer s.handlerWG.Done()
	for conn := range s.taskChan {
		s.handleClient(conn)
	}
}

func (s *STcp) handleClient(conn net.Conn) {
	connID := conn.RemoteAddr().String()
	entry := TCPConnectionEntry{Conn: conn}
	s.connections.Store(connID, entry)

	if s.config.OnConnected != nil {
		s.config.OnConnected(connID)
	}

	defer func() {
		conn.Close()
		s.connections.Delete(connID)
		if s.config.OnDisconnected != nil {
			s.config.OnDisconnected(connID)
		}
	}()

	for {
		var length uint32
		if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
			log.Printf("Failed to read data length: %v", err)
			return
		}
		data := make([]byte, length)
		if _, err := conn.Read(data); err != nil {
			log.Printf("Failed to read data: %v", err)
			return
		}
		if s.config.OnData != nil {
			s.config.OnData(connID, data)
		}
	}
}

func (s *STcp) Send(connectionID string, data []byte) {
	value, ok := s.connections.Load(connectionID)
	if !ok {
		return
	}
	entry := value.(TCPConnectionEntry)

	length := uint32(len(data))
	if err := binary.Write(entry.Conn, binary.BigEndian, length); err != nil {
		log.Printf("Failed to write data length: %v", err)
		return
	}
	if _, err := entry.Conn.Write(data); err != nil {
		log.Printf("Failed to write data: %v", err)
	}
}

func (s *STcp) Broadcast(data []byte, except ...map[string]bool) {
	s.connections.Range(func(key, value interface{}) bool {
		connID := key.(string)
		if len(except) > 0 {
			if _, ok := except[0][connID]; ok {
				return true
			}
		}
		s.Send(connID, data)
		return true
	})
}

func (s *STcp) IsConnection(connectionID string) bool {
	_, ok := s.connections.Load(connectionID)
	return ok
}

func (s *STcp) GetConnection(connectionID string) net.Conn {
	value, ok := s.connections.Load(connectionID)
	if !ok {
		return nil
	}
	return value.(TCPConnectionEntry).Conn
}

func (s *STcp) Shutdown() {
	if s.tcpListener != nil {
		s.tcpListener.Close()
	}
	s.acceptorWG.Wait()
	close(s.taskChan)
	s.handlerWG.Wait()
	s.connections.Range(func(key, value interface{}) bool {
		conn := value.(TCPConnectionEntry).Conn
		conn.Close()
		return true
	})
	s.connPool.CloseAll()
}
