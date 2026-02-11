# WebSocket 실시간 협업 구현 가이드

> **목표:** Liveblocks 대체, WebSocket 기반 실시간 Visual Editor 협업 구현
> **서버:** https://quel-canvas-server.onrender.com
> **환경변수:** `NEXT_PUBLIC_GO_SERVER_URL` (Vercel)

---

## 📋 개요

### 현재 상태
- ❌ Liveblocks 사용 중 (비용 발생, 외부 의존성)
- ✅ Go 서버에 WebSocket 엔드포인트 구현됨 (`/ws`)
- ✅ Render 서비스에서 WebSocket 지원

### 목표
- ✅ WebSocket 기반 실시간 협업 (Liveblocks 제거)
- ✅ 비용 절감 (~$10/월)
- ✅ 빠른 동기화 (300ms → 즉시)
- ✅ 완전한 제어권

---

## 🔧 환경변수 설정

### Vercel 환경변수
```bash
NEXT_PUBLIC_GO_SERVER_URL=https://quel-canvas-server.onrender.com
# 또는
NEXT_PUBLIC_CANVAS_SERVER_URL=https://quel-canvas-server.onrender.com
```

**확인 사항:**
- [ ] 두 환경변수 중 어떤 것을 사용하는지 확인
- [ ] 프로젝트 전체에서 일관되게 사용
- [ ] 로컬 `.env.local`에도 설정

### WebSocket URL 생성
```typescript
const GO_SERVER = process.env.NEXT_PUBLIC_GO_SERVER_URL || 'http://localhost:8080';
const wsUrl = `${GO_SERVER.replace('http', 'ws').replace('https', 'wss')}/ws`;
// 결과: wss://quel-canvas-server.onrender.com/ws
```

---

## 📁 파일 구조

```
src/app/[locale]/visual/
├── hooks/
│   ├── useSocketCollaboration.ts  // 🆕 생성 필요
│   └── useLiveblocks.ts           // ❌ 제거 예정
├── sockettest/
│   └── [category]/
│       └── page.tsx               // 🆕 테스트 페이지
└── [category]/
    └── page.tsx                   // ✏️ 수정 필요
```

---

## 🎯 구현 단계

### 1️⃣ Socket 훅 생성

**파일:** `src/app/[locale]/visual/hooks/useSocketCollaboration.ts`

```typescript
'use client';

import { useEffect, useState, useRef, useCallback } from 'react';
import { Node, Edge } from '@xyflow/react';

interface UseSocketCollaborationParams {
  enabled: boolean;
  orgId: string;
  workspaceId: string;
  memberId: string;
  userName: string;
  userColor: string;
  nodes: Node[];
  edges: Edge[];
  onNodesChange: (nodes: Node[]) => void;
  onEdgesChange: (edges: Edge[]) => void;
}

interface CollaborativeCursor {
  x: number;
  y: number;
  userName: string;
  userColor: string;
}

interface CollaborativeSelection {
  selectedNodeIds: string[];
  userName: string;
  userColor: string;
}

export function useSocketCollaboration({
  enabled,
  orgId,
  workspaceId,
  memberId,
  userName,
  userColor,
  nodes,
  edges,
  onNodesChange,
  onEdgesChange,
}: UseSocketCollaborationParams) {
  const [isConnected, setIsConnected] = useState(false);
  const [collaborativeCursors, setCollaborativeCursors] = useState<CollaborativeCursor[]>([]);
  const [collaborativeSelections, setCollaborativeSelections] = useState<CollaborativeSelection[]>([]);
  const wsRef = useRef<WebSocket | null>(null);
  const lastSentRef = useRef<string>('');
  const syncTimerRef = useRef<NodeJS.Timeout | null>(null);

  // WebSocket 연결
  useEffect(() => {
    if (!enabled || !orgId || !workspaceId || !memberId) return;

    const GO_SERVER = process.env.NEXT_PUBLIC_GO_SERVER_URL || 'http://localhost:8080';
    const wsUrl = `${GO_SERVER.replace('http', 'ws').replace('https', 'wss')}/ws?org_id=${orgId}&workspace_id=${workspaceId}&user_id=${memberId}&user_name=${encodeURIComponent(userName)}`;

    console.log('🔌 [WebSocket] Connecting to:', wsUrl);

    const ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      console.log('✅ [WebSocket] Connected');
      setIsConnected(true);

      // 초기 상태 요청
      ws.send(JSON.stringify({
        type: 'request-state'
      }));
    };

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        console.log('📨 [WebSocket] Received:', msg.type);

        switch (msg.type) {
          case 'initial-state':
            if (msg.data.nodes) onNodesChange(msg.data.nodes);
            if (msg.data.edges) onEdgesChange(msg.data.edges);
            console.log('📥 [WebSocket] Initial state loaded:', msg.data.nodes?.length, 'nodes');
            break;

          case 'nodes-updated':
            if (msg.user_id !== memberId) {
              console.log('🔄 [WebSocket] Remote nodes updated by', msg.user_name);
              onNodesChange(msg.data.nodes);
              onEdgesChange(msg.data.edges);
            }
            break;

          case 'cursor-update':
            if (msg.user_id !== memberId) {
              setCollaborativeCursors(prev => {
                const filtered = prev.filter(c => c.userName !== msg.user_name);
                return [...filtered, {
                  x: msg.data.x,
                  y: msg.data.y,
                  userName: msg.user_name,
                  userColor: msg.data.color
                }];
              });
            }
            break;

          case 'selection-update':
            if (msg.user_id !== memberId) {
              setCollaborativeSelections(prev => {
                const filtered = prev.filter(s => s.userName !== msg.user_name);
                return [...filtered, {
                  selectedNodeIds: msg.data.selectedNodeIds,
                  userName: msg.user_name,
                  userColor: msg.data.color
                }];
              });
            }
            break;

          case 'user-joined':
            console.log('👋 [WebSocket]', msg.user_name, 'joined');
            break;

          case 'user-left':
            console.log('👋 [WebSocket]', msg.user_name, 'left');
            setCollaborativeCursors(prev => prev.filter(c => c.userName !== msg.user_name));
            setCollaborativeSelections(prev => prev.filter(s => s.userName !== msg.user_name));
            break;
        }
      } catch (error) {
        console.error('❌ [WebSocket] Message parse error:', error);
      }
    };

    ws.onerror = (error) => {
      console.error('❌ [WebSocket] Error:', error);
      setIsConnected(false);
    };

    ws.onclose = () => {
      console.log('🔌 [WebSocket] Disconnected');
      setIsConnected(false);
    };

    wsRef.current = ws;

    return () => {
      if (wsRef.current) wsRef.current.close();
      if (syncTimerRef.current) clearTimeout(syncTimerRef.current);
    };
  }, [enabled, orgId, workspaceId, memberId, userName]);

  // 노드/엣지 변경 시 서버에 전송 (debounced)
  useEffect(() => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return;

    const currentState = JSON.stringify({
      nodes: nodes.map(n => ({ id: n.id, type: n.type, position: n.position, data: n.data })),
      edges: edges.map(e => ({ id: e.id, source: e.source, target: e.target })),
    });

    if (currentState === lastSentRef.current) return;

    if (syncTimerRef.current) clearTimeout(syncTimerRef.current);

    syncTimerRef.current = setTimeout(() => {
      if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
        console.log('📤 [WebSocket] Sending nodes update:', nodes.length, 'nodes');
        wsRef.current.send(JSON.stringify({
          type: 'sync-nodes',
          data: { nodes, edges }
        }));
        lastSentRef.current = currentState;
      }
    }, 300);
  }, [nodes, edges]);

  // 커서 전송
  const updateCursor = useCallback((x: number, y: number) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        type: 'cursor-update',
        data: { x, y, color: userColor }
      }));
    }
  }, [userColor]);

  // 선택 전송
  const updateSelection = useCallback((selectedNodeIds: string[]) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({
        type: 'selection-update',
        data: { selectedNodeIds, color: userColor }
      }));
    }
  }, [userColor]);

  return {
    isConnected,
    collaborativeCursors,
    collaborativeSelections,
    updateCursor,
    updateSelection,
  };
}
```

---

### 2️⃣ 테스트 페이지 생성

**파일:** `src/app/[locale]/visual/sockettest/[category]/page.tsx`

**기존 페이지 복사 후 수정:**

```typescript
// ❌ 제거
// import { useVisualLiveblocksWithSync } from '../hooks/useLiveblocks';

// ✅ 추가
import { useSocketCollaboration } from '../../hooks/useSocketCollaboration';

// ... 컴포넌트 내부 ...

// ❌ 기존 Liveblocks
const { collaborativeCursors, collaborativeSelections, ... } = useVisualLiveblocksWithSync({ ... });

// ✅ WebSocket으로 교체
const {
  isConnected,
  collaborativeCursors,
  collaborativeSelections,
  updateCursor,
  updateSelection,
} = useSocketCollaboration({
  enabled: true,
  orgId: orgId || '',
  workspaceId: workspaceId || '',
  memberId: userInfo?.quel_member_id || '',
  userName: userInfo?.quel_member_username || userInfo?.name || '',
  userColor: '#3B82F6',
  nodes,
  edges,
  onNodesChange: setNodes,
  onEdgesChange: setEdges,
});

// ✅ ReactFlow에 이벤트 추가
<ReactFlow
  nodes={nodes}
  edges={edges}
  onMouseMove={(event) => {
    if (isConnected) {
      const bounds = event.currentTarget.getBoundingClientRect();
      const x = event.clientX - bounds.left;
      const y = event.clientY - bounds.top;
      updateCursor(x, y);
    }
  }}
  onSelectionChange={(elements) => {
    const selectedIds = elements.nodes.map(n => n.id);
    updateSelection(selectedIds);
  }}
  // ... 기존 props
>
```

---

### 3️⃣ Go 서버 확인/수정

**파일:** `quel-canvas-server/main.go`

**Workspace별 Room 관리 확인:**

```go
type Client struct {
    Conn        *websocket.Conn
    OrgID       string
    WorkspaceID string  // ✅ 필수
    UserName    string
    UserID      string
}

// Room 키 생성
func getRoomKey(orgID, workspaceID string) string {
    return orgID + ":" + workspaceID
}

// 브로드캐스팅 시 workspace 체크
func handleBroadcast() {
    for {
        msg := <-broadcast

        clientsMu.Lock()
        for conn, client := range clients {
            // ✅ 같은 org + workspace만
            if client.OrgID == msg.OrgID && client.WorkspaceID == msg.WorkspaceID {
                err := conn.WriteJSON(msg)
                if err != nil {
                    log.Println("Write error:", err)
                    conn.Close()
                    delete(clients, conn)
                }
            }
        }
        clientsMu.Unlock()
    }
}
```

---

## 🧪 테스트 절차

### 1. 로컬 테스트

**URL 패턴:**
```
테스트: /ko-kr/visual/sockettest/fashion?org=[ORG_ID]&workspace=[WORKSPACE_ID]&member_id=[MEMBER_ID]
```

**첫 번째 브라우저:**
```
http://localhost:3000/ko-kr/visual/sockettest/fashion?org=cd88ae14-3c75-4dff-8012-7ac86580a365&workspace=19e840d4-3a52-4a7c-950e-608cc6ca1410&member_id=d36115bc-cba6-462a-a85c-92ec5b2b195f
```

**두 번째 브라우저 (시크릿):**
```
http://localhost:3000/ko-kr/visual/sockettest/fashion?org=cd88ae14-3c75-4dff-8012-7ac86580a365&workspace=19e840d4-3a52-4a7c-950e-608cc6ca1410&member_id=[다른_멤버_ID]
```

### 2. 테스트 체크리스트

- [ ] **연결:** 브라우저 콘솔에 "✅ [WebSocket] Connected"
- [ ] **노드 추가:** 한쪽에서 추가 → 다른쪽에서 실시간 표시
- [ ] **노드 이동:** 한쪽에서 이동 → 다른쪽에서 실시간 업데이트
- [ ] **노드 삭제:** 한쪽에서 삭제 → 다른쪽에서 실시간 제거
- [ ] **엣지 연결:** 한쪽에서 연결 → 다른쪽에서 실시간 표시
- [ ] **커서 이동:** 다른 사용자 커서 실시간 표시
- [ ] **노드 선택:** 다른 사용자 선택 상태 실시간 표시

### 3. 콘솔 로그 확인

**브라우저:**
```
🔌 [WebSocket] Connecting to: wss://...
✅ [WebSocket] Connected
📥 [WebSocket] Initial state loaded: X nodes
📤 [WebSocket] Sending nodes update: X nodes
📨 [WebSocket] Received: nodes-updated
```

**Go 서버 (Render Logs):**
```
✅ User [name] joined org [org_id] workspace [workspace_id]
📨 Broadcasting to X clients
❌ User [name] left
```

---

## 🚀 Production 배포

### 배포 전 체크리스트

- [ ] 로컬 테스트 모두 통과
- [ ] 3명 이상 동시 접속 테스트
- [ ] 노드 100개 이상 성능 테스트
- [ ] 네트워크 끊김 후 재연결 테스트
- [ ] CPU 사용률 70% 이하 확인
- [ ] 메모리 사용량 안정 확인

### 배포 순서

1. **Go 서버 배포** (Render)
   - 코드 푸시 → 자동 배포
   - Render 로그 확인

2. **Next.js 배포** (Vercel)
   - `/sockettest/[category]` 먼저 배포
   - Production 테스트

3. **기존 페이지 적용**
   - `/visual/[category]`에 WebSocket 적용
   - Liveblocks 제거

4. **패키지 정리**
   ```bash
   npm uninstall @liveblocks/client @liveblocks/react
   ```

---

## 📊 성능 모니터링

### Render 대시보드

- **CPU 사용률:** 70% 이하 유지
- **메모리:** 안정적
- **WebSocket 연결 수:** 활성 사용자 수
- **응답 시간:** 300ms 이하

### 최적화 팁

1. **Debounce 시간 조정:** 현재 300ms
2. **메시지 크기 최소화:** 필요한 데이터만 전송
3. **불필요한 브로드캐스트 제거:** 같은 사용자 제외

---

## ❌ 문제 해결

### WebSocket 연결 실패

```typescript
// 환경변수 확인
console.log('GO_SERVER:', process.env.NEXT_PUBLIC_GO_SERVER_URL);

// CORS 확인
// Go 서버 main.go에서 CheckOrigin 설정
```

### 동기화 안됨

```typescript
// 콘솔에서 메시지 송수신 확인
// 📤 Sending nodes update
// 📨 Received: nodes-updated
```

### Render 서버 메모리 부족

- WebSocket 연결 수 제한
- 비활성 연결 타임아웃 설정
- 메시지 크기 최적화

---

## 💰 비용 절감

**Before (Liveblocks):**
- 기본: $10/월
- 추가 사용자: 추가 비용

**After (WebSocket):**
- Render 기본 플랜: $0 (무료 티어)
- 또는 Render Pro: $7/월 (무제한)

**절감액:** 최소 $10/월 → 최대 $0/월

---

## ✅ 완료 후

1. ✅ Liveblocks 의존성 제거
2. ✅ 월 $10 비용 절감
3. ✅ 더 빠른 동기화 (즉시)
4. ✅ 완전한 제어권
5. ✅ 확장 가능한 구조

---

## 📝 참고 사항

### 현재 환경변수
- `NEXT_PUBLIC_GO_SERVER_URL`
- `NEXT_PUBLIC_CANVAS_SERVER_URL`

**⚠️ 주의:** 두 변수 중 하나로 통일 필요

### WebSocket URL 변환
```typescript
http://localhost:8080 → ws://localhost:8080/ws
https://quel-canvas-server.onrender.com → wss://quel-canvas-server.onrender.com/ws
```

### Render WebSocket 지원
- ✅ 자동 지원
- ✅ HTTPS → WSS 자동 변환
- ✅ 추가 설정 불필요
