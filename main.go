package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"quel-canvas-server/modules/common/config"
	klingmigration "quel-canvas-server/modules/kling-migration"
	landingdemo "quel-canvas-server/modules/landing-demo"
	"quel-canvas-server/modules/modify"
	"quel-canvas-server/modules/multiview"
	"quel-canvas-server/modules/preview"
	"quel-canvas-server/modules/submodule/nanobanana"
	"quel-canvas-server/modules/unified-prompt/landing"
	"quel-canvas-server/modules/unified-prompt/studio"
	"quel-canvas-server/modules/worker"
	fluxschnell "quel-canvas-server/modules/submodule/flux-schnell"
	"quel-canvas-server/modules/submodule/seedream"

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
	EnableCompression: true, // WebSocket 압축 활성화
}

// 연결된 클라이언트 정보
type Client struct {
	conn        *websocket.Conn
	orgId       string
	workspaceId string
	userId      string
	userName    string
	userInfo    map[string]interface{}
	send        chan []byte
}

// 세션 관리 (Room으로 사용)
type Session struct {
	id           string
	clients      map[string]*Client
	mutex        sync.RWMutex
	createdAt    time.Time
	lastActivity time.Time

	// Visual Editor 상태 저장 (협업용)
	nodes        []interface{}          // React Flow nodes
	edges        []interface{}          // React Flow edges
	lastSyncBy   string                 // 마지막으로 상태를 동기화한 사용자 ID
	lastSyncAt   time.Time              // 마지막 동기화 시간
}

// 세션 매니저
type SessionManager struct {
	sessions map[string]*Session
	mutex    sync.RWMutex
	metrics  *ServerMetrics
}

// 서버 메트릭
type ServerMetrics struct {
	TotalSessions    int       `json:"totalSessions"`
	ActiveSessions   int       `json:"activeSessions"`
	TotalConnections int       `json:"totalConnections"`
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
	Type           string                 `json:"type"`
	SessionId      string                 `json:"sessionId"` // 기존 호환성 유지
	UserId         string                 `json:"userId"`
	UserInfo       map[string]interface{} `json:"userInfo"`
	ItemIds        []string               `json:"itemIds,omitempty"`
	SectionIds     []string               `json:"sectionIds,omitempty"`
	ItemUpdates    map[string]interface{} `json:"itemUpdates,omitempty"`
	SectionUpdates map[string]interface{} `json:"sectionUpdates,omitempty"`
	ItemId         string                 `json:"itemId,omitempty"`
	SectionId      string                 `json:"sectionId,omitempty"`
	Label          string                 `json:"label,omitempty"`
	Title          string                 `json:"title,omitempty"`

	// 캔버스 관련 필드들
	CanvasItems []interface{} `json:"canvasItems,omitempty"` // 캔버스 아이템들
	Sections    []interface{} `json:"sections,omitempty"`    // 섹션들
	CursorX     float64       `json:"cursorX,omitempty"`     // 마우스 커서 X
	CursorY     float64       `json:"cursorY,omitempty"`     // 마우스 커서 Y
	IsHost      bool          `json:"isHost,omitempty"`      // 호스트 여부

	// Creation History 관련 필드들
	ShowCreationHistory bool          `json:"showCreationHistory,omitempty"` // 히스토리 표시 여부
	HostProductions     []interface{} `json:"hostProductions,omitempty"`     // 호스트의 프로덕션 데이터

	// Visual Editor 협업 관련 필드들 (신규)
	OrgId       string                 `json:"org_id,omitempty"`       // 조직 ID
	WorkspaceId string                 `json:"workspace_id,omitempty"` // 워크스페이스 ID
	UserName    string                 `json:"user_name,omitempty"`    // 사용자 이름
	Data        map[string]interface{} `json:"data,omitempty"`         // 범용 데이터 (nodes, edges 등)
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

		log.Printf("Created new session: %s (Total: %d, Active: %d)",
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

	log.Printf("Client %s joined session %s (Clients: %d, Total Connections: %d)",
		client.userId, s.id, clientCount, sessionManager.metrics.TotalConnections)

	// user_joined 메시지를 모든 클라이언트에게 브로드캐스트 (mutex 해제 후)
	joinMessage := Message{
		Type:        "user_joined",
		UserId:      client.userId,
		UserName:    client.userName,
		UserInfo:    client.userInfo,
		SessionId:   s.id,
		OrgId:       client.orgId,
		WorkspaceId: client.workspaceId,
	}
	s.broadcastToAll(joinMessage)
	log.Printf("📢 Broadcasted user_joined for %s (%s) to all clients in room %s", client.userName, client.userId, s.id)
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
			Type:        "user_left",
			UserId:      userId,
			UserName:    client.userName,
			OrgId:       client.orgId,
			WorkspaceId: client.workspaceId,
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

			log.Printf("Cleaned up empty session: %s", sessionId)
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
				log.Printf("Disconnecting client %s from expired session %s", userId, sessionId)
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

	log.Printf("Started session cleanup routines (Empty: 5min, Expired: 30min)")
}

// WebSocket 핸들러
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// WebSocket 연결 업그레이드
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	// URL 파라미터 추출
	orgId := r.URL.Query().Get("org_id")
	workspaceId := r.URL.Query().Get("workspace_id")
	userId := r.URL.Query().Get("user_id")
	userName := r.URL.Query().Get("user_name")

	if orgId == "" || workspaceId == "" || userId == "" {
		log.Printf("❌ Missing required parameters (org_id, workspace_id, user_id)")
		conn.Close()
		return
	}

	if userName == "" {
		userName = "Unknown User"
	}

	// Room 키 생성 (org_id:workspace_id)
	roomKey := orgId + ":" + workspaceId

	// 클라이언트 생성
	client := &Client{
		conn:        conn,
		orgId:       orgId,
		workspaceId: workspaceId,
		userId:      userId,
		userName:    userName,
		send:        make(chan []byte, 1024), // 버퍼 크기 증가 (256 → 1024)
	}

	log.Printf("✅ [WebSocket] New connection - Org: %s, Workspace: %s, User: %s (%s)", orgId, workspaceId, userName, userId)

	// Room에 클라이언트 추가
	session := sessionManager.getOrCreateSession(roomKey)

	// 현재 Room의 사용자 수 확인
	session.mutex.RLock()
	existingUsers := len(session.clients)
	session.mutex.RUnlock()

	log.Printf("📊 [WebSocket] Room %s has %d existing users", roomKey, existingUsers)

	session.addClient(client)

	// 고루틴으로 읽기/쓰기 처리
	go client.writePump()
	go client.readPump(session)
}

// Ping/Pong 설정
const (
	pongWait   = 60 * time.Second    // Pong 대기 시간
	pingPeriod = (pongWait * 9) / 10 // Ping 전송 주기 (54초)
	writeWait  = 10 * time.Second    // Write 타임아웃
)

// 클라이언트로부터 메시지 읽기
func (c *Client) readPump(session *Session) {
	defer func() {
		session.removeClient(c.userId)
		c.conn.Close()
	}()

	// Pong 핸들러 설정
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

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
			log.Printf("Host %s updated history visibility to: %v (productions: %d)",
				c.userId, message.ShowCreationHistory, len(message.HostProductions))

		case "user_joined":
			log.Printf("User %s joined session %s", c.userId, message.SessionId)

		// Visual Editor 협업 메시지 타입 (신규)
		case "request-state":
			log.Printf("📥 [WebSocket] User %s (%s) requested initial state", c.userName, c.userId)

			// Room에 저장된 상태 읽기
			session.mutex.RLock()
			nodes := session.nodes
			edges := session.edges
			lastSyncBy := session.lastSyncBy
			lastSyncAt := session.lastSyncAt
			session.mutex.RUnlock()

			// 초기 상태 응답
			initialState := Message{
				Type: "initial-state",
				Data: map[string]interface{}{
					"nodes":      nodes,
					"edges":      edges,
					"lastSyncBy": lastSyncBy,
					"lastSyncAt": lastSyncAt,
				},
				OrgId:       c.orgId,
				WorkspaceId: c.workspaceId,
			}

			// 요청한 사용자에게만 전송
			if stateBytes, err := json.Marshal(initialState); err == nil {
				select {
				case c.send <- stateBytes:
					log.Printf("✅ [WebSocket] Sent initial state to %s (%d nodes, %d edges)",
						c.userName, len(nodes), len(edges))
				default:
					log.Printf("⚠️ [WebSocket] Failed to send initial state to %s (channel full)", c.userName)
				}
			}

			// 이 메시지는 브로드캐스트하지 않음 (continue로 건너뜀)
			continue

		case "sync-nodes":
			// Room 상태 업데이트
			if message.Data != nil {
				session.mutex.Lock()
				if nodes, ok := message.Data["nodes"].([]interface{}); ok {
					session.nodes = nodes
				}
				if edges, ok := message.Data["edges"].([]interface{}); ok {
					session.edges = edges
				}
				session.lastSyncBy = c.userId
				session.lastSyncAt = time.Now()
				nodeCount := len(session.nodes)
				edgeCount := len(session.edges)
				session.mutex.Unlock()

				log.Printf("📤 [WebSocket] User %s (%s) synced state (%d nodes, %d edges)",
					c.userName, c.userId, nodeCount, edgeCount)
			}

			// 메시지에 발신자 정보 추가
			message.OrgId = c.orgId
			message.WorkspaceId = c.workspaceId
			message.UserName = c.userName
			message.Type = "nodes-updated" // 브로드캐스트용 타입 변경

		case "cursor-update":
			// 커서 업데이트는 로깅하지 않음 (성능)
			message.OrgId = c.orgId
			message.WorkspaceId = c.workspaceId
			message.UserName = c.userName

		case "selection-update":
			// 선택 업데이트는 로깅하지 않음 (성능)
			message.OrgId = c.orgId
			message.WorkspaceId = c.workspaceId
			message.UserName = c.userName

		case "user-leave":
			log.Printf("👋 [WebSocket] User %s (%s) is leaving gracefully", c.userName, c.userId)

			// user-left 브로드캐스트 (다른 사용자들에게 알림)
			leaveMessage := Message{
				Type:        "user-left",
				UserId:      c.userId,
				UserName:    c.userName,
				OrgId:       c.orgId,
				WorkspaceId: c.workspaceId,
			}
			session.broadcastToOthers(c.userId, leaveMessage)

			// 클라이언트 제거 및 연결 종료
			session.removeClient(c.userId)
			c.conn.Close()
			return // readPump 종료
		}

		// 메시지 타입에 따라 브로드캐스트 방식 결정
		switch message.Type {
		case "user_joined", "request_canvas_state", "user_left":
			// 이 메시지들은 모든 사용자에게 전송 (호스트 포함)
			session.broadcastToAll(message)
		case "nodes-updated", "cursor-update", "selection-update":
			// Visual Editor 협업 메시지는 자신을 제외한 다른 사용자에게만 전송
			session.broadcastToOthers(c.userId, message)
		default:
			// 나머지는 자신을 제외한 다른 사용자에게만 전송
			session.broadcastToOthers(c.userId, message)
		}
	}
}

// 클라이언트로 메시지 쓰기
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// 채널이 닫혔으면 연결 종료
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}

		case <-ticker.C:
			// 주기적으로 Ping 전송
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
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
		"status":  "healthy",
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
	totalSessions := sessionManager.metrics.TotalSessions
	activeSessions := sessionManager.metrics.ActiveSessions
	totalConnections := sessionManager.metrics.TotalConnections
	startTime := sessionManager.metrics.StartTime
	sessionManager.metrics.mutex.RUnlock()

	// 추가 정보 계산
	uptime := time.Since(startTime)

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
			"startTime":        startTime,
			"totalSessions":    totalSessions,
			"activeSessions":   activeSessions,
			"totalConnections": totalConnections,
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
	if _, err := config.LoadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 정리 루틴 시작
	sessionManager.startCleanupRoutine()

	// Redis Queue Worker 시작 (백그라운드)
	go worker.StartWorker()

	// Kling Video Worker 시작 (백그라운드)
	klingWorker := klingmigration.NewWorker()
	if klingWorker != nil {
		go klingWorker.StartWorker()
		log.Println("✅ Kling Video Worker started")
	} else {
		log.Println("⚠️ Kling Video Worker not started - check KLING_AI keys")
	}

	// Worker 모듈 초기화 완료

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

	// Modify 모듈 라우트 등록
	modifyHandler := modify.NewModifyHandler()
	if modifyHandler != nil {
		modifyHandler.RegisterRoutes(r)
	} else {
		log.Println("Failed to initialize Modify handler")
	}

	// Preview 라우트 등록 (슬래시 노드 프리뷰 용도)
	previewHandler := preview.NewPreviewHandler()
	if previewHandler != nil {
		previewHandler.RegisterRoutes(r)
	} else {
		log.Println("Failed to initialize Preview handler")
	}

	// Cancel API 라우트 등록
	cancelHandler := worker.NewCancelHandler()
	if cancelHandler != nil {
		cancelHandler.RegisterRoutes(r)
	} else {
		log.Println("Failed to initialize Cancel handler")
	}

	// Enqueue API 라우트 등록 (Vercel → Go Server → Redis)
	enqueueHandler := worker.NewEnqueueHandler()
	if enqueueHandler != nil {
		enqueueHandler.RegisterRoutes(r)
	} else {
		log.Println("⚠️ Failed to initialize Enqueue handler - check Redis connection")
	}

	// Unified Prompt - Landing 라우트 등록
	landingHandler := landing.NewHandler()
	if landingHandler != nil {
		r.HandleFunc("/api/unified-prompt/landing/generate", landingHandler.HandleGenerate).Methods("POST", "OPTIONS")
		r.HandleFunc("/api/unified-prompt/landing/check-limit", landingHandler.HandleCheckLimit).Methods("GET", "OPTIONS")
		log.Println("✅ Unified Prompt Landing routes registered")
	} else {
		log.Println("⚠️ Failed to initialize Landing handler")
	}

	// Unified Prompt - Studio 라우트 등록
	studioHandler := studio.NewHandler()
	if studioHandler != nil {
		r.HandleFunc("/api/unified-prompt/studio/generate", studioHandler.HandleGenerate).Methods("POST", "OPTIONS")
		r.HandleFunc("/api/unified-prompt/studio/check-credits", studioHandler.HandleCheckCredits).Methods("GET", "OPTIONS")
		r.HandleFunc("/api/unified-prompt/studio/analyze", studioHandler.HandleAnalyze).Methods("POST", "OPTIONS")
		log.Println("✅ Unified Prompt Studio routes registered")
	} else {
		log.Println("⚠️ Failed to initialize Studio handler")
	}

	// Landing Demo 라우트 등록 (체험존 - 무제한)
	landingDemoHandler := landingdemo.NewHandler()
	if landingDemoHandler != nil {
		r.HandleFunc("/api/landing-demo/generate", landingDemoHandler.HandleGenerate).Methods("POST", "OPTIONS")
		log.Println("✅ Landing Demo routes registered")
	} else {
		log.Println("⚠️ Failed to initialize Landing Demo handler")
	}

	// Multiview 360 라우트 등록
	multiviewHandler := multiview.NewHandler()
	if multiviewHandler != nil {
		multiviewHandler.RegisterRoutes(r)
	} else {
		log.Println("⚠️ Failed to initialize Multiview handler")
	}

	// Nanobanana (Gemini) 라우트 등록 - 랜딩 템플릿용 + 이미지 분석
	nanobananaHandler := nanobanana.NewHandler()
	if nanobananaHandler != nil {
		r.HandleFunc("/api/nanobanana/generate", nanobananaHandler.HandleGenerate).Methods("POST", "OPTIONS")
		r.HandleFunc("/api/nanobanana/analyze", nanobananaHandler.HandleAnalyze).Methods("POST", "OPTIONS")
		log.Println("✅ Nanobanana routes registered (generate + analyze)")
	} else {
		log.Println("⚠️ Failed to initialize Nanobanana handler")
	}

	// Flux Schnell 라우트 등록 - Dream 모드용 빠른 이미지 생성
	fluxSchnellHandler := fluxschnell.NewHandler()
	if fluxSchnellHandler != nil {
		r.HandleFunc("/api/flux-schnell/generate", fluxSchnellHandler.HandleGenerate).Methods("POST", "OPTIONS")
		log.Println("✅ Flux Schnell routes registered")
	} else {
		log.Println("⚠️ Failed to initialize Flux Schnell handler - check RUNWARE_API_KEY")
	}

	// Seedream 라우트 등록 - 랜딩 페이지용 고품질 이미지 생성 (Seedream 3.0)
	seedreamHandler := seedream.NewHandler()
	if seedreamHandler != nil {
		r.HandleFunc("/api/seedream/generate", seedreamHandler.HandleGenerate).Methods("POST", "OPTIONS")
		log.Println("✅ Seedream routes registered")
	} else {
		log.Println("⚠️ Failed to initialize Seedream handler - check RUNWARE_API_KEY")
	}

	// Kling Migration 라우트 등록 - Image to Video (Kling AI)
	klingHandler := klingmigration.NewHandler()
	if klingHandler != nil {
		klingHandler.RegisterRoutes(r)
	} else {
		log.Println("⚠️ Failed to initialize Kling handler - check KLING_AI keys")
	}

	// 포트 설정 (Render.com은 PORT 환경변수 사용)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Quel Canvas Collaboration Server starting on port %s", port)
	log.Printf("WebSocket endpoint: ws://localhost:%s/ws", port)
	log.Printf("Health check: http://localhost:%s/health", port)
	log.Printf("Metrics: http://localhost:%s/metrics", port)
	log.Printf("Admin cleanup: http://localhost:%s/admin/cleanup", port)
	log.Printf("Modify submit: http://localhost:%s/api/modify/submit", port)
	log.Printf("Modify status: http://localhost:%s/api/modify/status/{jobId}", port)
	log.Printf("Job cancel: http://localhost:%s/api/jobs/{jobId}/cancel", port)
	log.Printf("Job enqueue: http://localhost:%s/enqueue", port)
	log.Printf("Unified Prompt Landing: http://localhost:%s/api/unified-prompt/landing/generate", port)
	log.Printf("Unified Prompt Studio: http://localhost:%s/api/unified-prompt/studio/generate", port)
	log.Printf("Landing Demo: http://localhost:%s/api/landing-demo/generate", port)
	log.Printf("Multiview 360: http://localhost:%s/api/multiview/generate", port)
	log.Printf("Nanobanana: http://localhost:%s/api/nanobanana/generate", port)
	log.Printf("Nanobanana Analyze: http://localhost:%s/api/nanobanana/analyze", port)
	log.Printf("Kling Video Enqueue: http://localhost:%s/enqueue-video", port)

	// 서버 시작
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
