package kubeagentservice

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// AgentSession holds the communication channels between a frontend WS and a remote agent
type AgentSession struct {
	Token          string
	Commands       chan string
	Results        chan string
	AgentConnected bool
	CreatedAt      time.Time
	mu             sync.Mutex

	// Phase 3: direct WebSocket connection references for remediation forwarding
	agentConn    *websocket.Conn
	frontendConn *websocket.Conn
}

var (
	sessions   = make(map[string]*AgentSession)
	sessionsMu sync.RWMutex
)

// GetOrCreateSession returns an existing session or creates a new one for the given token
func GetOrCreateSession(token string) *AgentSession {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	if sess, ok := sessions[token]; ok {
		return sess
	}

	sess := &AgentSession{
		Token:     token,
		Commands:  make(chan string, 10),
		Results:   make(chan string, 100),
		CreatedAt: time.Now(),
	}
	sessions[token] = sess

	// Auto-cleanup after 15 minutes
	go func() {
		time.Sleep(15 * time.Minute)
		sessionsMu.Lock()
		delete(sessions, token)
		sessionsMu.Unlock()
	}()

	return sess
}

// GetSession returns an existing session or nil
func GetSession(token string) *AgentSession {
	sessionsMu.RLock()
	defer sessionsMu.RUnlock()
	return sessions[token]
}

// SetAgentConnected marks the agent as connected
func (s *AgentSession) SetAgentConnected(connected bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AgentConnected = connected
}

// IsAgentConnected returns whether the agent is connected
func (s *AgentSession) IsAgentConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.AgentConnected
}

// ─── Phase 3: WebSocket connection getters/setters ───────────────────────────

// SetAgentConn stores the agent's WebSocket connection in the session
func (s *AgentSession) SetAgentConn(conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentConn = conn
}

// GetAgentConn returns the agent's WebSocket connection
func (s *AgentSession) GetAgentConn() *websocket.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.agentConn
}

// SetFrontendConn stores the frontend terminal's WebSocket connection
func (s *AgentSession) SetFrontendConn(conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frontendConn = conn
}

// GetFrontendConn returns the frontend terminal's WebSocket connection
func (s *AgentSession) GetFrontendConn() *websocket.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.frontendConn
}
