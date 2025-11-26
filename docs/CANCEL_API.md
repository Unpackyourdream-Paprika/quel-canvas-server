# Job Cancel API

이미지 생성 작업을 중간에 취소할 수 있는 API입니다.

## 엔드포인트

```
POST /api/jobs/{jobId}/cancel
```

## 요청

### Path Parameters

| 파라미터 | 타입 | 필수 | 설명 |
|---------|------|-----|------|
| `jobId` | string | ✅ | 취소할 Job의 UUID |

### Headers

```
Content-Type: application/json
```

### 요청 예시

```bash
curl -X POST https://your-server.com/api/jobs/550e8400-e29b-41d4-a716-446655440000/cancel
```

## 응답

### 성공 응답 (200 OK)

```json
{
  "success": true,
  "message": "Cancel request sent. Job will stop after current image.",
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "current_status": "processing",
  "completed_images": 3,
  "total_images": 10
}
```

### 이미 완료/취소된 Job (200 OK)

```json
{
  "success": false,
  "message": "Job already completed",
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "job_status": "completed",
  "completed_images": 10
}
```

### 에러 응답

**Job을 찾을 수 없음 (404 Not Found)**
```json
{
  "error": "Job not found"
}
```

**jobId 누락 (400 Bad Request)**
```json
{
  "error": "jobId is required"
}
```

## 동작 방식

```
┌─────────────────────────────────────────────────────────────┐
│                      Cancel 흐름                            │
└─────────────────────────────────────────────────────────────┘

1. 클라이언트가 Cancel API 호출
        │
        ▼
2. Redis에 취소 플래그 설정
   └─ key: "job:{jobId}:cancelled" = "true"
   └─ TTL: 1시간 (자동 만료)
        │
        ▼
3. Worker가 이미지 생성 루프에서 취소 플래그 체크
   └─ 새 이미지 생성 전마다 Redis 조회
   └─ 취소 플래그가 있으면 루프 중단
        │
        ▼
4. 이미 생성된 이미지는 유지
   └─ generated_attach_ids 배열에 저장된 것은 그대로
   └─ completed_images 카운트도 유지
        │
        ▼
5. Job 상태를 "cancelled"로 변경
   └─ job_status = "cancelled"
   └─ 취소 시점까지의 결과물 보존
```

## Job 상태 값

| 상태 | 설명 |
|-----|------|
| `pending` | 대기 중 |
| `processing` | 처리 중 |
| `completed` | 완료 |
| `failed` | 실패 |
| `user_cancelled` | 사용자가 취소함 |

## Next.js 클라이언트 예시

### API 함수

```typescript
// lib/api/jobs.ts

export async function cancelJob(jobId: string) {
  const response = await fetch(
    `${process.env.NEXT_PUBLIC_API_URL}/api/jobs/${jobId}/cancel`,
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
    }
  );

  if (!response.ok) {
    throw new Error('Failed to cancel job');
  }

  return response.json();
}
```

### React Hook 예시

```typescript
// hooks/useJobCancel.ts

import { useState } from 'react';
import { cancelJob } from '@/lib/api/jobs';

export function useJobCancel() {
  const [isCancelling, setIsCancelling] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const cancel = async (jobId: string) => {
    setIsCancelling(true);
    setError(null);

    try {
      const result = await cancelJob(jobId);

      if (result.success) {
        // 취소 성공 - 이미 생성된 이미지들은 유지됨
        console.log(`Job cancelled. ${result.completed_images} images saved.`);
        return result;
      } else {
        // 이미 완료/취소된 상태
        console.log(result.message);
        return result;
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
      throw err;
    } finally {
      setIsCancelling(false);
    }
  };

  return { cancel, isCancelling, error };
}
```

### 컴포넌트 사용 예시

```tsx
// components/ImageGenerationProgress.tsx

import { useJobCancel } from '@/hooks/useJobCancel';

interface Props {
  jobId: string;
  completedImages: number;
  totalImages: number;
  status: string;
  onCancelled: () => void;
}

export function ImageGenerationProgress({
  jobId,
  completedImages,
  totalImages,
  status,
  onCancelled
}: Props) {
  const { cancel, isCancelling } = useJobCancel();

  const handleCancel = async () => {
    if (confirm('진행 중인 이미지 생성을 취소하시겠습니까?\n이미 생성된 이미지는 유지됩니다.')) {
      try {
        await cancel(jobId);
        onCancelled();
      } catch (error) {
        alert('취소 요청에 실패했습니다.');
      }
    }
  };

  return (
    <div className="p-4 border rounded-lg">
      <div className="flex justify-between items-center mb-2">
        <span>이미지 생성 중...</span>
        <span>{completedImages} / {totalImages}</span>
      </div>

      <div className="w-full bg-gray-200 rounded-full h-2 mb-4">
        <div
          className="bg-blue-600 h-2 rounded-full transition-all"
          style={{ width: `${(completedImages / totalImages) * 100}%` }}
        />
      </div>

      {status === 'processing' && (
        <button
          onClick={handleCancel}
          disabled={isCancelling}
          className="px-4 py-2 bg-red-500 text-white rounded hover:bg-red-600 disabled:opacity-50"
        >
          {isCancelling ? '취소 중...' : '생성 취소'}
        </button>
      )}
    </div>
  );
}
```

## 주의사항

1. **취소는 즉시 반영되지 않습니다**
   - 현재 생성 중인 이미지가 완료된 후에 중단됩니다
   - API 호출에서 실제 중단까지 약간의 딜레이가 있을 수 있습니다

2. **이미 생성된 이미지는 삭제되지 않습니다**
   - 취소 시점까지 생성된 이미지들은 `generated_attach_ids`에 저장됩니다
   - 클라이언트에서 이 이미지들을 표시할 수 있습니다

3. **취소 플래그는 1시간 후 자동 만료됩니다**
   - Redis TTL이 1시간으로 설정되어 있습니다
   - Job이 이미 종료된 후에는 플래그가 있어도 영향 없습니다

4. **병렬 처리 중인 Job**
   - 여러 조합이 병렬로 처리되는 경우, 각 goroutine이 취소를 감지합니다
   - 모든 병렬 작업이 중단될 때까지 약간의 시간이 소요될 수 있습니다
