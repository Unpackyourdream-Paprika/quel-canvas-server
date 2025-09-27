# Quel Canvas Collaboration Server

Canvas collaboration을 위한 WebSocket 서버입니다.

## 기능

- 실시간 WebSocket 통신
- 다중 사용자 세션 관리
- 객체 선택 상태 동기화
- 드래그 위치 실시간 업데이트
- 라벨 텍스트 동기화

## 로컬 실행

```bash
go mod tidy
go run main.go
```

서버가 `http://localhost:8080`에서 실행됩니다.

## Render.com 배포

1. 이 프로젝트를 GitHub repository에 push
2. Render.com에서 "New Web Service" 생성
3. GitHub repository 연결
4. 자동 배포 대기

## API 엔드포인트

- `GET /` - 헬스 체크
- `GET /health` - 헬스 체크
- `GET /session/{sessionId}` - 세션 정보 조회
- `WS /ws?session={sessionId}&user={userId}` - WebSocket 연결

## WebSocket 메시지 타입

### 클라이언트 → 서버

```json
{
  "type": "user_selection",
  "sessionId": "session123",
  "userId": "user456",
  "userInfo": {
    "name": "John Doe",
    "email": "john@example.com"
  },
  "itemIds": ["item1", "item2"],
  "sectionIds": ["section1"]
}
```

```json
{
  "type": "item_position_update",
  "itemUpdates": {
    "item1": { "position": { "x": 100, "y": 200 } }
  }
}
```

```json
{
  "type": "label_update",
  "itemId": "item1",
  "label": "New Label"
}
```

### 서버 → 클라이언트

- 동일한 형식으로 다른 클라이언트들에게 브로드캐스트
- `user_left` 타입으로 사용자 퇴장 알림

## 환경 변수

- `PORT` - 서버 포트 (기본값: 8080, Render.com에서 자동 설정)

## CORS

개발용으로 모든 origin을 허용합니다. 프로덕션에서는 특정 도메인만 허용하도록 수정하세요.