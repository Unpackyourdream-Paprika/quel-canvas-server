package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	generateimage "quel-canvas-server/modules/generate-image"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// 개발용 - 모든 origin 허용
		// 프로덕션에서는 특정 도메인만 허용하도록 수정
		return true
	},
}

// 연결된 클라이언트 정보
type Client struct {
	conn      *websocket.Conn
	sessionId string
	userId    string
	userInfo  map[string]interface{}
	send      chan []byte
}

// 세션 관리
type Session struct {
	id        string
	clients   map[string]*Client
	mutex     sync.RWMutex
	createdAt time.Time
	lastActivity time.Time
}

// 세션 매니저
type SessionManager struct {
	sessions map[string]*Session
	mutex    sync.RWMutex
	metrics  *ServerMetrics
}

// 서버 메트릭
type ServerMetrics struct {
	TotalSessions    int `json:"totalSessions"`
	ActiveSessions   int `json:"activeSessions"`
	TotalConnections int `json:"totalConnections"`
	StartTime        time.Time `json:"startTime"`
	mutex            sync.RWMutex
}

var sessionManager = &SessionManager{
	sessions: make(map[string]*Session),
	metrics: &ServerMetrics{
		StartTime: time.Now(),
	},
}

// 메시지 타입
type Message struct {
	Type        string                 `json:"type"`
	SessionId   string                 `json:"sessionId"`
	UserId      string                 `json:"userId"`
	UserInfo    map[string]interface{} `json:"userInfo"`
	ItemIds     []string               `json:"itemIds,omitempty"`
	SectionIds  []string               `json:"sectionIds,omitempty"`
	ItemUpdates map[string]interface{} `json:"itemUpdates,omitempty"`
	SectionUpdates map[string]interface{} `json:"sectionUpdates,omitempty"`
	ItemId      string                 `json:"itemId,omitempty"`
	SectionId   string                 `json:"sectionId,omitempty"`
	Label       string                 `json:"label,omitempty"`
	Title       string                 `json:"title,omitempty"`

	// 새로운 필드들
	CanvasItems []interface{}          `json:"canvasItems,omitempty"`    // 캔버스 아이템들
	Sections    []interface{}          `json:"sections,omitempty"`       // 섹션들
	CursorX     float64                `json:"cursorX,omitempty"`        // 마우스 커서 X
	CursorY     float64                `json:"cursorY,omitempty"`        // 마우스 커서 Y
	IsHost      bool                   `json:"isHost,omitempty"`         // 호스트 여부

	// Creation History 관련 필드들
	ShowCreationHistory bool          `json:"showCreationHistory,omitempty"` // 히스토리 표시 여부
	HostProductions     []interface{} `json:"hostProductions,omitempty"`     // 호스트의 프로덕션 데이터
}

// 세션 가져오기 또는 생성
func (sm *SessionManager) getOrCreateSession(sessionId string) *Session {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session, exists := sm.sessions[sessionId]
	if !exists {
		now := time.Now()
		session = &Session{
			id:           sessionId,
			clients:      make(map[string]*Client),
			createdAt:    now,
			lastActivity: now,
		}
		sm.sessions[sessionId] = session

		// 메트릭 업데이트
		sm.metrics.mutex.Lock()
		sm.metrics.TotalSessions++
		sm.metrics.ActiveSessions++
		sm.metrics.mutex.Unlock()

		log.Printf("✅ Created new session: %s (Total: %d, Active: %d)",
			sessionId, sm.metrics.TotalSessions, sm.metrics.ActiveSessions)
	}

	// 활동 시간 업데이트
	session.lastActivity = time.Now()
	return session
}

// 클라이언트를 세션에 추가
func (s *Session) addClient(client *Client) {
	s.mutex.Lock()
	s.clients[client.userId] = client
	s.lastActivity = time.Now()
	clientCount := len(s.clients)
	s.mutex.Unlock()

	// 메트릭 업데이트
	sessionManager.metrics.mutex.Lock()
	sessionManager.metrics.TotalConnections++
	sessionManager.metrics.mutex.Unlock()

	log.Printf("👤 Client %s joined session %s (Clients: %d, Total Connections: %d)",
		client.userId, s.id, clientCount, sessionManager.metrics.TotalConnections)

	// user_joined 메시지를 모든 클라이언트에게 브로드캐스트 (mutex 해제 후)
	joinMessage := Message{
		Type:      "user_joined",
		UserId:    client.userId,
		UserInfo:  client.userInfo,
		SessionId: s.id,
	}
	s.broadcastToAll(joinMessage)
	log.Printf("📢 Broadcasted user_joined for %s to all clients in session %s", client.userId, s.id)
}

// 클라이언트를 세션에서 제거
func (s *Session) removeClient(userId string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if client, exists := s.clients[userId]; exists {
		close(client.send)
		delete(s.clients, userId)
		s.lastActivity = time.Now()

		log.Printf("👋 Client %s left session %s (Remaining: %d)", userId, s.id, len(s.clients))

		// 다른 클라이언트들에게 사용자 퇴장 알림
		userLeftMsg := Message{
			Type:   "user_left",
			UserId: userId,
		}
		s.broadcastToOthers(userId, userLeftMsg)

		// 세션이 비어있으면 정리 스케줄링
		if len(s.clients) == 0 {
			log.Printf("🗑️  Session %s is now empty, will be cleaned up", s.id)
		}
	}
}

// 다른 클라이언트들에게 메시지 브로드캐스트
func (s *Session) broadcastToOthers(senderUserId string, message Message) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	for userId, client := range s.clients {
		if userId != senderUserId {
			select {
			case client.send <- messageBytes:
			default:
				close(client.send)
				delete(s.clients, userId)
			}
		}
	}
}

// 모든 클라이언트에게 메시지 브로드캐스트 (자신 포함)
func (s *Session) broadcastToAll(message Message) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	for userId, client := range s.clients {
		select {
		case client.send <- messageBytes:
			if message.Type == "history_visibility_update" {
				log.Printf("📤 Sent history_visibility_update to user %s (showCreationHistory: %v, productions: %d)",
					userId, message.ShowCreationHistory, len(message.HostProductions))
			} else {
				log.Printf("Sent message type '%s' to user %s", message.Type, userId)
			}
		default:
			close(client.send)
			delete(s.clients, userId)
		}
	}
}

// 빈 세션 정리
func (sm *SessionManager) cleanupEmptySessions() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	cleaned := 0
	for sessionId, session := range sm.sessions {
		session.mutex.RLock()
		isEmpty := len(session.clients) == 0
		session.mutex.RUnlock()

		if isEmpty {
			delete(sm.sessions, sessionId)
			cleaned++

			// 메트릭 업데이트
			sm.metrics.mutex.Lock()
			sm.metrics.ActiveSessions--
			sm.metrics.mutex.Unlock()

			log.Printf("🧹 Cleaned up empty session: %s", sessionId)
		}
	}

	if cleaned > 0 {
		log.Printf("🗑️  Cleaned up %d empty sessions (Active: %d)", cleaned, sm.metrics.ActiveSessions)
	}
}

// 만료된 세션 정리 (24시간 후)
func (sm *SessionManager) cleanupExpiredSessions() {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	now := time.Now()
	expiredThreshold := 24 * time.Hour
	inactiveThreshold := 2 * time.Hour

	cleaned := 0
	for sessionId, session := range sm.sessions {
		session.mutex.RLock()
		isExpired := now.Sub(session.createdAt) > expiredThreshold
		isInactive := now.Sub(session.lastActivity) > inactiveThreshold && len(session.clients) == 0
		session.mutex.RUnlock()

		if isExpired || isInactive {
			// 연결된 클라이언트들 정리
			session.mutex.Lock()
			for userId, client := range session.clients {
				close(client.send)
				log.Printf("🔌 Disconnecting client %s from expired session %s", userId, sessionId)
			}
			session.mutex.Unlock()

			delete(sm.sessions, sessionId)
			cleaned++

			// 메트릭 업데이트
			sm.metrics.mutex.Lock()
			sm.metrics.ActiveSessions--
			sm.metrics.mutex.Unlock()

			reason := "expired"
			if isInactive {
				reason = "inactive"
			}
			log.Printf("⏰ Cleaned up %s session: %s (Age: %v, Inactive: %v)",
				reason, sessionId, now.Sub(session.createdAt), now.Sub(session.lastActivity))
		}
	}

	if cleaned > 0 {
		log.Printf("🧼 Cleaned up %d expired/inactive sessions (Active: %d)", cleaned, sm.metrics.ActiveSessions)
	}
}

// 정기적 정리 작업 시작
func (sm *SessionManager) startCleanupRoutine() {
	// 5분마다 빈 세션 정리
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			sm.cleanupEmptySessions()
		}
	}()

	// 30분마다 만료된 세션 정리
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			sm.cleanupExpiredSessions()
		}
	}()

	log.Printf("🔄 Started session cleanup routines (Empty: 5min, Expired: 30min)")
}

// WebSocket 핸들러
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// WebSocket 연결 업그레이드
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// URL 파라미터에서 세션 ID와 사용자 ID 추출
	sessionId := r.URL.Query().Get("session")
	userId := r.URL.Query().Get("user")

	if sessionId == "" || userId == "" {
		log.Printf("Missing session or user parameter")
		conn.Close()
		return
	}

	// 클라이언트 생성
	client := &Client{
		conn:      conn,
		sessionId: sessionId,
		userId:    userId,
		send:      make(chan []byte, 256),
	}

	log.Printf("🔍 New WebSocket connection - Session: %s, User: %s", sessionId, userId)

	// 세션에 클라이언트 추가
	session := sessionManager.getOrCreateSession(sessionId)

	// 현재 세션의 사용자 수 확인
	session.mutex.RLock()
	existingUsers := len(session.clients)
	session.mutex.RUnlock()

	log.Printf("📊 Session %s has %d existing users before adding new user", sessionId, existingUsers)

	session.addClient(client)

	// 고루틴으로 읽기/쓰기 처리
	go client.writePump()
	go client.readPump(session)
}

// 클라이언트로부터 메시지 읽기
func (c *Client) readPump(session *Session) {
	defer func() {
		session.removeClient(c.userId)
		c.conn.Close()
	}()

	for {
		var message Message
		err := c.conn.ReadJSON(&message)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// 메시지 타입에 따른 처리
		switch message.Type {
		case "user_selection":
			// 선택 업데이트는 로깅하지 않음 (성능 최적화)

		case "item_position_update":
			log.Printf("User %s updated item positions", c.userId)

		case "section_position_update":
			log.Printf("User %s updated section positions", c.userId)

		case "label_update":
			log.Printf("User %s updated label", c.userId)

		case "cursor_move":
			// 커서 움직임은 로깅하지 않음 (성능 최적화)

		case "request_canvas_state":
			log.Printf("User %s requested canvas state", c.userId)
			// 호스트에게 캔버스 상태 요청 전달 - 모든 사용자에게 브로드캐스트
			message.UserId = c.userId // 요청자 ID 설정

		case "canvas_state_response":
			log.Printf("Host %s sent canvas state with %d items, %d sections",
				c.userId, len(message.CanvasItems), len(message.Sections))

		case "canvas_items_update":
			log.Printf("User %s updated canvas items (count: %d)", c.userId, len(message.CanvasItems))

		case "sections_update":
			log.Printf("User %s updated sections (count: %d)", c.userId, len(message.Sections))

		case "history_visibility_update":
			log.Printf("📊 Host %s updated history visibility to: %v (productions: %d)",
				c.userId, message.ShowCreationHistory, len(message.HostProductions))

		case "user_joined":
			log.Printf("User %s joined session %s", c.userId, message.SessionId)
		}

		// 메시지 타입에 따라 브로드캐스트 방식 결정
		switch message.Type {
		case "user_joined", "request_canvas_state", "user_left":
			// 이 메시지들은 모든 사용자에게 전송 (호스트 포함)
			session.broadcastToAll(message)
		default:
			// 나머지는 자신을 제외한 다른 사용자에게만 전송
			session.broadcastToOthers(c.userId, message)
		}
	}
}

// 클라이언트로 메시지 쓰기
func (c *Client) writePump() {
	defer c.conn.Close()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}
		}
	}
}

// CORS 헤더 추가
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// 헬스 체크 엔드포인트
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"service": "quel-canvas-collaboration",
	})
}

// 세션 정보 조회 엔드포인트
func getSessionInfo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionId := vars["sessionId"]

	sessionManager.mutex.RLock()
	session, exists := sessionManager.sessions[sessionId]
	sessionManager.mutex.RUnlock()

	if !exists {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Session not found",
		})
		return
	}

	session.mutex.RLock()
	clientCount := len(session.clients)
	clientIds := make([]string, 0, len(session.clients))
	for userId := range session.clients {
		clientIds = append(clientIds, userId)
	}
	session.mutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessionId":    sessionId,
		"clientCount":  clientCount,
		"clients":      clientIds,
		"createdAt":    session.createdAt,
		"lastActivity": session.lastActivity,
		"age":          time.Since(session.createdAt).String(),
		"inactive":     time.Since(session.lastActivity).String(),
	})
}

// 서버 메트릭 조회 엔드포인트
func getMetrics(w http.ResponseWriter, r *http.Request) {
	sessionManager.metrics.mutex.RLock()
	metrics := *sessionManager.metrics
	sessionManager.metrics.mutex.RUnlock()

	// 추가 정보 계산
	uptime := time.Since(metrics.StartTime)

	sessionManager.mutex.RLock()
	sessionDetails := make([]map[string]interface{}, 0, len(sessionManager.sessions))
	totalClients := 0

	for sessionId, session := range sessionManager.sessions {
		session.mutex.RLock()
		clientCount := len(session.clients)
		totalClients += clientCount

		sessionDetails = append(sessionDetails, map[string]interface{}{
			"sessionId":    sessionId,
			"clientCount":  clientCount,
			"createdAt":    session.createdAt,
			"lastActivity": session.lastActivity,
			"age":          time.Since(session.createdAt).String(),
			"inactive":     time.Since(session.lastActivity).String(),
		})
		session.mutex.RUnlock()
	}
	sessionManager.mutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"server": map[string]interface{}{
			"uptime":           uptime.String(),
			"startTime":        metrics.StartTime,
			"totalSessions":    metrics.TotalSessions,
			"activeSessions":   metrics.ActiveSessions,
			"totalConnections": metrics.TotalConnections,
			"currentClients":   totalClients,
		},
		"sessions": sessionDetails,
	})
}

// 모든 세션 강제 정리 (관리자용)
func forceCleanupSessions(w http.ResponseWriter, r *http.Request) {
	// 즉시 빈 세션 정리
	sessionManager.cleanupEmptySessions()

	// 즉시 만료된 세션 정리
	sessionManager.cleanupExpiredSessions()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "Cleanup completed",
	})
}

func main() {
	// 환경변수 로드
	if _, err := generateimage.LoadConfig(); err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	// 정리 루틴 시작
	sessionManager.startCleanupRoutine()

	// Redis Queue Worker 시작 (백그라운드)
	go generateimage.StartWorker()

   // Generate Image 모듈 초기화




	// 라우터 설정
	r := mux.NewRouter()

	// CORS 미들웨어 적용
	r.Use(enableCORS)

	// 라우트 설정
	r.HandleFunc("/", healthCheck).Methods("GET")
	r.HandleFunc("/health", healthCheck).Methods("GET")
	r.HandleFunc("/ws", handleWebSocket)
	r.HandleFunc("/session/{sessionId}", getSessionInfo).Methods("GET")
	r.HandleFunc("/metrics", getMetrics).Methods("GET")
	r.HandleFunc("/admin/cleanup", forceCleanupSessions).Methods("POST")






	// 포트 설정 (Render.com은 PORT 환경변수 사용)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 Quel Canvas Collaboration Server starting on port %s", port)
	log.Printf("📡 WebSocket endpoint: ws://localhost:%s/ws", port)
	log.Printf("❤️  Health check: http://localhost:%s/health", port)
	log.Printf("📊 Metrics: http://localhost:%s/metrics", port)
	log.Printf("🧹 Admin cleanup: http://localhost:%s/admin/cleanup", port)

	// 서버 시작
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}