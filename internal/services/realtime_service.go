package services

import (
	"boot-whatsapp-golang/pkg/logger"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

const (
	realtimeSendBuffer  = 128
	realtimeWriteTO     = 10 * time.Second
	realtimePingEvery   = 30 * time.Second
	realtimeMaxReadSize = 1024
)

type RealtimeEnvelope struct {
	Event      string      `json:"event"`
	SessionKey string      `json:"session_key,omitempty"`
	TenantID   string      `json:"tenant_id,omitempty"`
	Timestamp  time.Time   `json:"timestamp"`
	Data       interface{} `json:"data,omitempty"`
}

type realtimeClient struct {
	id        string
	conn      *websocket.Conn
	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once
	scopes    []string
	remoteIP  string
}

type RealtimeService struct {
	log *logger.Logger

	mu           sync.RWMutex
	clients      map[string]*realtimeClient
	clientsByKey map[string]map[string]*realtimeClient
}

func NewRealtimeService(log *logger.Logger) *RealtimeService {
	return &RealtimeService{
		log:          log,
		clients:      make(map[string]*realtimeClient),
		clientsByKey: make(map[string]map[string]*realtimeClient),
	}
}

func (s *RealtimeService) ServeWS(w http.ResponseWriter, r *http.Request, scopes []string) error {
	normalizedScopes := normalizeScopes(scopes...)
	if len(normalizedScopes) == 0 {
		http.Error(w, "at least one websocket scope is required", http.StatusBadRequest)
		return fmt.Errorf("nenhum escopo informado para websocket")
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return fmt.Errorf("falha ao aceitar websocket: %w", err)
	}
	conn.SetReadLimit(realtimeMaxReadSize)

	client := &realtimeClient{
		id:       uuid.NewString(),
		conn:     conn,
		send:     make(chan []byte, realtimeSendBuffer),
		done:     make(chan struct{}),
		scopes:   normalizedScopes,
		remoteIP: r.RemoteAddr,
	}

	s.registerClient(client)
	s.log.Infof("WS conectado (id=%s, scopes=%v, remote=%s)", client.id, client.scopes, client.remoteIP)

	connectedPayload := RealtimeEnvelope{
		Event:     "ws.connected",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"client_id": client.id,
			"scopes":    client.scopes,
		},
	}
	if raw, marshalErr := json.Marshal(connectedPayload); marshalErr == nil {
		s.enqueue(client, raw)
	}

	go s.writeLoop(client)
	s.readLoop(client)
	s.removeClient(client, websocket.StatusNormalClosure, "client disconnected")

	return nil
}

func (s *RealtimeService) Publish(eventType, sessionKey, tenantID string, data interface{}, scopes ...string) {
	if strings.TrimSpace(eventType) == "" {
		return
	}

	targetScopes := normalizeScopes(scopes...)
	if len(targetScopes) == 0 {
		targetScopes = normalizeScopes(sessionKey, tenantID)
	}
	if len(targetScopes) == 0 {
		return
	}

	envelope := RealtimeEnvelope{
		Event:      strings.TrimSpace(eventType),
		SessionKey: strings.TrimSpace(sessionKey),
		TenantID:   strings.TrimSpace(tenantID),
		Timestamp:  time.Now(),
		Data:       data,
	}

	payload, err := json.Marshal(envelope)
	if err != nil {
		s.log.Warnf("Falha ao serializar evento realtime %s: %v", eventType, err)
		return
	}

	for _, client := range s.snapshotClients(targetScopes) {
		if !s.enqueue(client, payload) {
			s.log.Warnf("WS fila cheia; desconectando cliente %s", client.id)
			s.removeClient(client, websocket.StatusPolicyViolation, "send buffer full")
		}
	}
}

func (s *RealtimeService) registerClient(client *realtimeClient) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.clients[client.id] = client
	for _, scope := range client.scopes {
		if s.clientsByKey[scope] == nil {
			s.clientsByKey[scope] = make(map[string]*realtimeClient)
		}
		s.clientsByKey[scope][client.id] = client
	}
}

func (s *RealtimeService) removeClient(client *realtimeClient, code websocket.StatusCode, reason string) {
	client.closeOnce.Do(func() {
		close(client.done)

		s.mu.Lock()
		delete(s.clients, client.id)
		for _, scope := range client.scopes {
			scopeClients := s.clientsByKey[scope]
			if scopeClients == nil {
				continue
			}
			delete(scopeClients, client.id)
			if len(scopeClients) == 0 {
				delete(s.clientsByKey, scope)
			}
		}
		s.mu.Unlock()

		_ = client.conn.Close(code, reason)
		s.log.Infof("WS desconectado (id=%s, scopes=%v, code=%d)", client.id, client.scopes, code)
	})
}

func (s *RealtimeService) enqueue(client *realtimeClient, payload []byte) bool {
	select {
	case <-client.done:
		return false
	default:
	}

	select {
	case client.send <- payload:
		return true
	default:
		return false
	}
}

func (s *RealtimeService) snapshotClients(scopes []string) []*realtimeClient {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := make(map[string]struct{}, 64)
	clients := make([]*realtimeClient, 0, 64)

	for _, scope := range scopes {
		scopeClients := s.clientsByKey[scope]
		for id, client := range scopeClients {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			clients = append(clients, client)
		}
	}

	return clients
}

func (s *RealtimeService) writeLoop(client *realtimeClient) {
	ticker := time.NewTicker(realtimePingEvery)
	defer ticker.Stop()

	for {
		select {
		case <-client.done:
			return
		case payload := <-client.send:
			ctx, cancel := context.WithTimeout(context.Background(), realtimeWriteTO)
			err := client.conn.Write(ctx, websocket.MessageText, payload)
			cancel()
			if err != nil {
				s.log.Warnf("Falha ao escrever WS para cliente %s: %v", client.id, err)
				s.removeClient(client, websocket.StatusGoingAway, "write failed")
				return
			}
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), realtimeWriteTO)
			err := client.conn.Ping(ctx)
			cancel()
			if err != nil {
				s.log.Warnf("Falha no ping WS para cliente %s: %v", client.id, err)
				s.removeClient(client, websocket.StatusGoingAway, "ping failed")
				return
			}
		}
	}
}

func (s *RealtimeService) readLoop(client *realtimeClient) {
	for {
		select {
		case <-client.done:
			return
		default:
		}

		if _, _, err := client.conn.Read(context.Background()); err != nil {
			return
		}
	}
}

func normalizeScopes(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))

	for _, value := range values {
		v := strings.TrimSpace(value)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}

	return out
}
