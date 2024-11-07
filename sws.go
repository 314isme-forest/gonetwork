package gonetwork

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

type SWs struct {
	config      ServerConfig
	httpServer  *http.Server
	connections sync.Map
	handlerWG   sync.WaitGroup
}

type WSConnectionEntry struct {
	Conn *websocket.Conn
}

func WSNewS(config ServerConfig) *SWs {
	server := &SWs{
		config: config,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleConnections)
	var tlsConfig *tls.Config
	if config.Secure {
		cert, err := TLSCert(config.Domain)
		if err != nil {
			log.Printf("Failed to load certificate: %v", err)
			return nil
		}
		tlsConfig = &tls.Config{Certificates: []tls.Certificate{cert}, InsecureSkipVerify: true}
	} else {
		tlsConfig = &tls.Config{InsecureSkipVerify: true}
	}

	server.httpServer = &http.Server{
		Addr:      config.Address + ":" + config.WSPort,
		TLSConfig: tlsConfig,
		Handler:   mux,
	}
	return server
}

func (s *SWs) Listen(ctx context.Context) error {
	s.handlerWG.Add(1)
	go func() {
		defer s.handlerWG.Done()
		if s.config.Secure {
			if err := s.httpServer.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
				log.Printf("Failed to start secure server: %v", err)
			}
		} else {
			if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
				log.Printf("Failed to start server: %v", err)
			}
		}
	}()
	return nil
}

func (s *SWs) handleConnections(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		http.Error(w, "Failed to accept websocket connection", http.StatusInternalServerError)
		return
	}

	connID := r.RemoteAddr
	entry := WSConnectionEntry{Conn: conn}
	s.connections.Store(connID, entry)

	if s.config.OnConnected != nil {
		s.config.OnConnected(connID)
	}

	s.handlerWG.Add(1)
	go s.handleClient(connID)
}

func (s *SWs) handleClient(connID string) {
	defer s.handlerWG.Done()

	entry, ok := s.connections.Load(connID)
	if !ok {
		return
	}
	connEntry := entry.(WSConnectionEntry)

	defer func() {
		connEntry.Conn.Close(websocket.StatusNormalClosure, "")
		s.connections.Delete(connID)
		if s.config.OnDisconnected != nil {
			s.config.OnDisconnected(connID)
		}
	}()

	for {
		_, data, err := connEntry.Conn.Read(context.Background())
		if err != nil {
			log.Printf("Failed to read data: %v", err)
			return
		}
		if s.config.OnData != nil {
			s.config.OnData(connID, data)
		}
	}
}

func (s *SWs) Send(connectionID string, data []byte) {
	value, ok := s.connections.Load(connectionID)
	if !ok {
		return
	}
	entry := value.(WSConnectionEntry)

	if err := entry.Conn.Write(context.Background(), websocket.MessageBinary, data); err != nil {
		log.Printf("Failed to send data: %v", err)
	}
}

func (s *SWs) Broadcast(data []byte, except ...map[string]bool) {
	s.connections.Range(func(key, value interface{}) bool {
		connID := key.(string)
		if len(except) > 0 && except[0][connID] {
			return true
		}
		s.Send(connID, data)
		return true
	})
}

func (s *SWs) IsConnection(connectionID string) bool {
	_, ok := s.connections.Load(connectionID)
	return ok
}

func (s *SWs) GetConnection(connectionID string) *websocket.Conn {
	value, ok := s.connections.Load(connectionID)
	if !ok {
		return nil
	}
	return value.(WSConnectionEntry).Conn
}

func (s *SWs) Shutdown() {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.httpServer.Shutdown(ctx)
	}
	s.handlerWG.Wait()
	s.connections.Range(func(key, value interface{}) bool {
		conn := value.(WSConnectionEntry).Conn
		conn.Close(websocket.StatusNormalClosure, "")
		return true
	})
}
