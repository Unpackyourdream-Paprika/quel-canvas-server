# 백엔드 스키마 업데이트: Eats 카테고리용 `isPreEdited` 필드

## 개요
**eats 카테고리 전용**으로 새로운 선택적 boolean 필드 `isPreEdited`가 job 페이로드 구조에 추가되었습니다. 이 필드는 업로드된 이미지가 생성 워크플로우에 사용되기 전에 미리 보정/색보정이 되었는지를 나타냅니다.

## 영향받는 구조체

### 1. 싱글체인 Job Input Data
**위치**: jobs 테이블의 `job_input_data` 컬럼

**변경 전**:
```json
{
  "basePrompt": "...",
  "individualImageAttachIds": [...],
  "mergedImageAttachId": 123,
  "combinations": [...],
  "userId": "...",
  "category": "eats",
  "globalContext": {...},
  "aspectRatio": "1:1",
  "aspect-ratio": "1:1"
}
```

**변경 후** (eats 카테고리 전용):
```json
{
  "basePrompt": "...",
  "individualImageAttachIds": [...],
  "mergedImageAttachId": 123,
  "combinations": [...],
  "userId": "...",
  "category": "eats",
  "globalContext": {...},
  "aspectRatio": "1:1",
  "aspect-ratio": "1:1",
  "isPreEdited": false  // ✅ 새로 추가된 필드 (eats 카테고리 전용)
}
```

### 2. 멀티스테이지 파이프라인 Job Input Data
**위치**: jobs 테이블의 `job_input_data.stages[]` 배열

**변경 전**:
```json
{
  "stages": [
    {
      "stage_index": 0,
      "quantity": 1,
      "prompt": "...",
      "negative_prompt": "...",
      "individualImageAttachIds": [...],
      "cameraAngle": "front",
      "shotType": "full body",
      "aspect-ratio": "1:1",
      "globalContext": {...},
      "mergedImageAttachId": 123
    }
  ],
  "userId": "...",
  "category": "eats",
  "totalImages": 1
}
```

**변경 후** (eats 카테고리 전용):
```json
{
  "stages": [
    {
      "stage_index": 0,
      "quantity": 1,
      "prompt": "...",
      "negative_prompt": "...",
      "individualImageAttachIds": [...],
      "cameraAngle": "front",
      "shotType": "full body",
      "aspect-ratio": "1:1",
      "globalContext": {...},
      "mergedImageAttachId": 123,
      "isPreEdited": true  // ✅ 새로 추가된 필드 (eats 카테고리 전용)
    }
  ],
  "userId": "...",
  "category": "eats",
  "totalImages": 1
}
```

## 필드 명세

| 속성 | 타입 | 필수 여부 | 기본값 | 카테고리 범위 |
|------|------|----------|--------|--------------|
| `isPreEdited` | `boolean` | 아니오 | `false` | **eats 전용** |

### 필드 동작
- **표시되는 경우**: `category === "eats"` 일 때만
- **기본값**: `false` (토글이 꺼져있거나 설정되지 않은 경우)
- **가능한 값**: `true` 또는 `false`
- **다른 카테고리 영향**: 없음 - beauty, fashion, cinema 등 다른 카테고리에는 이 필드가 **포함되지 않음**

## Go Struct 업데이트 권장사항

### 싱글체인 Job Input
```go
type JobInputData struct {
    BasePrompt              string                 `json:"basePrompt"`
    IndividualImageAttachIds []ImageAttachInfo     `json:"individualImageAttachIds"`
    MergedImageAttachId     *int                   `json:"mergedImageAttachId,omitempty"`
    Combinations            []AngleShotCombination `json:"combinations"`
    UserId                  string                 `json:"userId"`
    Category                string                 `json:"category"`
    GlobalContext           *GlobalContext         `json:"globalContext,omitempty"`
    AspectRatio             string                 `json:"aspectRatio"`
    AspectRatioDash         string                 `json:"aspect-ratio"`

    // ✅ 신규: eats 카테고리에만 존재
    IsPreEdited             *bool                  `json:"isPreEdited,omitempty"`
}
```

### 멀티스테이지 파이프라인 Stage Data
```go
type StageData struct {
    StageIndex              int                    `json:"stage_index"`
    Quantity                int                    `json:"quantity"`
    Prompt                  string                 `json:"prompt"`
    NegativePrompt          string                 `json:"negative_prompt"`
    IndividualImageAttachIds []ImageAttachInfo     `json:"individualImageAttachIds"`
    CameraAngle             string                 `json:"cameraAngle"`
    ShotType                string                 `json:"shotType"`
    AspectRatio             string                 `json:"aspect-ratio"`
    GlobalContext           *GlobalContext         `json:"globalContext,omitempty"`
    MergedImageAttachId     *int                   `json:"mergedImageAttachId,omitempty"`

    // ✅ 신규: eats 카테고리에만 존재
    IsPreEdited             *bool                  `json:"isPreEdited,omitempty"`
}
```

## 사용 예시

### 예시 1: isPreEdited = true인 싱글체인 (eats 카테고리)
```json
{
  "stageName": "Single Chain",
  "totalImages": 4,
  "jobInputData": {
    "basePrompt": "front, full body. 신선한 채소가 들어간 맛있는 한국식 비빔밥. 중요: 분할 레이아웃 금지, 그리드 레이아웃 금지. 각 이미지는 제품을 강조하는 단일 통합 구성이어야 함.",
    "individualImageAttachIds": [
      { "attachId": 12345, "type": "food" }
    ],
    "mergedImageAttachId": null,
    "combinations": [
      { "angle": "front", "shot": "full body", "quantity": 1 },
      { "angle": "side", "shot": "close up", "quantity": 1 }
    ],
    "userId": "user_abc123",
    "category": "eats",
    "globalContext": null,
    "aspectRatio": "1:1",
    "aspect-ratio": "1:1",
    "isPreEdited": true
  }
}
```

### 예시 2: isPreEdited 값이 혼합된 멀티스테이지 (eats 카테고리)
```json
{
  "stages": [
    {
      "stage_index": 0,
      "quantity": 2,
      "prompt": "신선한 바질이 올라간 아름답게 플레이팅된 파스타 요리",
      "negative_prompt": "",
      "individualImageAttachIds": [
        { "attachId": 11111, "type": "food" }
      ],
      "cameraAngle": "top down",
      "shotType": "full body",
      "aspect-ratio": "1:1",
      "globalContext": null,
      "mergedImageAttachId": null,
      "isPreEdited": true
    },
    {
      "stage_index": 1,
      "quantity": 1,
      "prompt": "다채로운 채소가 들어간 신선한 샐러드",
      "negative_prompt": "",
      "individualImageAttachIds": [
        { "attachId": 22222, "type": "food" }
      ],
      "cameraAngle": "front",
      "shotType": "close up",
      "aspect-ratio": "16:9",
      "globalContext": null,
      "mergedImageAttachId": null,
      "isPreEdited": false
    }
  ],
  "userId": "user_xyz789",
  "category": "eats",
  "totalImages": 3
}
```

### 예시 3: Beauty 카테고리 (isPreEdited 필드 없음)
```json
{
  "stageName": "Single Chain",
  "totalImages": 1,
  "jobInputData": {
    "basePrompt": "front, full body. 립스틱을 바르는 모델. 중요: 분할 레이아웃 금지...",
    "individualImageAttachIds": [
      { "attachId": 99999, "type": "product" }
    ],
    "combinations": [...],
    "userId": "user_beauty",
    "category": "beauty",
    "aspectRatio": "1:1",
    "aspect-ratio": "1:1"
    // ❌ beauty 카테고리에는 isPreEdited 필드 없음
  }
}
```

## 백엔드 처리 가이드라인

### 필드 읽기
```go
// 안전한 접근 패턴
if jobInput.Category == "eats" && jobInput.IsPreEdited != nil {
    isPreEdited := *jobInput.IsPreEdited

    if isPreEdited {
        // 이미지가 미리 보정/색보정됨
        // 색보정 또는 스타일 전이 파라미터 조정이 필요할 수 있음
        log.Printf("스테이지에서 보정된 이미지 사용 중")
    } else {
        // 이미지가 원본/미편집 상태
        // 표준 처리 적용
        log.Printf("스테이지에서 원본 이미지 사용 중")
    }
}

// 멀티스테이지의 경우
for _, stage := range jobInput.Stages {
    if jobInput.Category == "eats" && stage.IsPreEdited != nil {
        if *stage.IsPreEdited {
            // 이 스테이지는 보정된 이미지를 사용함
        }
    }
}
```

### 하위 호환성
- 필드가 `omitempty` JSON 태그를 사용하므로 설정되지 않은 경우 JSON에 나타나지 않음
- 레거시 job(이 업데이트 이전에 생성된)에는 이 필드가 없음
- Go 코드는 역참조하기 전에 `nil` 체크를 해야 함
- 필드가 없을 때의 기본 동작은 `isPreEdited: false`와 동일해야 함

```go
// 안전한 접근을 위한 헬퍼 함수
func IsPreEditedImage(jobInput *JobInputData) bool {
    if jobInput.Category != "eats" {
        return false
    }
    if jobInput.IsPreEdited == nil {
        return false // 레거시 job의 경우 기본값 false
    }
    return *jobInput.IsPreEdited
}
```

## 테스트 체크리스트

- [ ] `isPreEdited: true`인 싱글체인 job 검증 (eats 카테고리)
- [ ] `isPreEdited: false`인 싱글체인 job 검증 (eats 카테고리)
- [ ] `isPreEdited` 값이 혼합된 멀티스테이지 job 검증 (eats 카테고리)
- [ ] beauty 카테고리 job에 `isPreEdited` 필드가 포함되지 않음을 검증
- [ ] fashion 카테고리 job에 `isPreEdited` 필드가 포함되지 않음을 검증
- [ ] cinema 카테고리 job에 `isPreEdited` 필드가 포함되지 않음을 검증
- [ ] 레거시 job(필드 없음)과의 하위 호환성 테스트
- [ ] Go worker 코드에서 nil 포인터 안전성 검증

## 프론트엔드 구현 세부사항

### UI 위치
- **컴포넌트**: `GroupNode.tsx` (Asset 노드)
- **표시 여부**: `/visual/eats` 워크스페이스에서만 표시됨
- **UI 요소**: 경고 텍스트가 있는 iOS 스타일 토글 스위치
- **번역**:
  - 한국어: "보정 완료된 이미지" / "편집된 이미지는 생성 결과의 색감/스타일에 영향을 줄 수 있어요"
  - 영어: "Pre-edited Image" / "Edited images may affect the color/style of generation results"
  - 일본어: "編集済み画像" / "編集された画像は生成結果の色調/スタイルに影響を与える可能性があります"

### 데이터 저장
- 값은 `useWorkspacePersistence`를 통해 워크스페이스 localStorage/IndexedDB에 저장됨
- `node.data.isPreEdited` 필드에 저장됨
- 저장된 상태에서 워크스페이스를 불러올 때 복원됨

### 렌더 플로우
1. 사용자가 eats 워크스페이스의 Asset 노드에 이미지를 업로드
2. 사용자가 "보정 완료된 이미지" 스위치를 토글 (on/off)
3. 사용자가 Render 버튼 클릭
4. 프론트엔드가 그룹 노드 데이터에서 `isPreEdited` 추출
5. 카테고리가 "eats"인 경우에만 job 페이로드에 필드가 조건부로 추가됨
6. 플래그와 함께 Go worker에 job이 제출됨

## 마이그레이션 참고사항

### 데이터베이스
데이터베이스 마이그레이션 불필요. 런타임 JSON 필드 추가입니다.

### API 버전 관리
해당 없음 - 추가적이며 하위 호환됩니다.

### 롤백 계획
이 기능을 롤백해야 하는 경우:
1. 프론트엔드: `GroupNode.tsx`에서 토글 UI 제거
2. 프론트엔드: job 페이로드 빌더에서 `isPreEdited` 필드 제거
3. 백엔드: 존재하는 경우 `isPreEdited` 필드 무시 (struct 정의는 남겨두어도 안전)

## 문의
이 스키마 변경에 대한 질문은 프론트엔드 팀에 문의하세요.

**구현 날짜**: 2025-12-20
**프론트엔드 PR**: [추가 예정]
**백엔드 PR**: [추가 예정]
