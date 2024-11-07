package gonetwork

import (
	"context"
	"log"

	"google.golang.org/protobuf/proto"
)

type Server struct {
	config    ServerConfig
	tcpServer *STcp
	wsServer  *SWs
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewServer(config ServerConfig) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		config:    config,
		tcpServer: TCPNewS(config),
		wsServer:  WSNewS(config),
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (s *Server) Listen() {
	if s.config.TCPPort != "" {
		go func() {
			if err := s.tcpServer.Listen(s.ctx); err != nil {
				log.Printf("TCP Server Listen error: %v", err)
			} else {
				if s.config.Secure {
					log.Printf("TCP Server is now listening on %s:%s with TLS", s.config.Address, s.config.TCPPort)
				} else {
					log.Printf("TCP Server is now listening on %s:%s", s.config.Address, s.config.TCPPort)
				}
			}
		}()
	}
	if s.config.WSPort != "" {
		go func() {
			if err := s.wsServer.Listen(s.ctx); err != nil {
				log.Printf("WS Server Listen error: %v", err)
			} else {
				if s.config.Secure {
					log.Printf("WS Server is now listening on %s:%s with TLS", s.config.Address, s.config.WSPort)
				} else {
					log.Printf("WS Server is now listening on %s:%s", s.config.Address, s.config.WSPort)
				}
			}
		}()
	}
}

func (s *Server) Broadcast(dataHandler, dataType string, data proto.Message, except ...map[string]bool) {
	dataByte := Encode(dataHandler, dataType, data)
	if s.config.TCPPort != "" {
		s.tcpServer.Broadcast(dataByte, except...)
	}
	if s.config.WSPort != "" {
		s.wsServer.Broadcast(dataByte, except...)
	}
}

func (s *Server) Send(connectionID string, dataHandler, dataType string, data proto.Message) {
	dataByte := Encode(dataHandler, dataType, data)
	if s.config.TCPPort != "" && s.tcpServer.IsConnection(connectionID) {
		s.tcpServer.Send(connectionID, dataByte)
	}
	if s.config.WSPort != "" && s.wsServer.IsConnection(connectionID) {
		s.wsServer.Send(connectionID, dataByte)
	}
}

func (s *Server) GetConnection(connectionID string) interface{} {
	if s.config.TCPPort != "" && s.tcpServer.IsConnection(connectionID) {
		if conn := s.tcpServer.GetConnection(connectionID); conn != nil {
			return conn
		}
	}
	if s.config.WSPort != "" && s.wsServer.IsConnection(connectionID) {
		if conn := s.wsServer.GetConnection(connectionID); conn != nil {
			return conn
		}
	}
	return nil
}

func (s *Server) Shutdown() {
	s.cancel()
	s.tcpServer.Shutdown()
	s.wsServer.Shutdown()
}
