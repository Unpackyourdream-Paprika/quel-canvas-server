# Redis Job Processing & Database Query Analysis
## Quel Canvas Server - Complete Data Flow

---

## 1. REDIS JOB CONSUMER/PROCESSOR CODE

### Entry Point: Worker Process
**File:** `/Users/s2s2hyun/Desktop/quel-canvas-server/modules/fashion/worker.go` (Lines 20-65)

```go
// StartWorker - Redis Queue Worker 시작
func StartWorker() {
    // 1. Connect to Redis
    rdb := connectRedis(cfg)  // Lines 945-981
    
    // 2. Watch queue: "jobs:queue" (BRPOP - Blocking Right Pop)
    for {
        result, err := rdb.BRPop(ctx, 0, "jobs:queue").Result()
        if err != nil {
            log.Printf("Redis BRPOP error: %v", err)
            time.Sleep(5 * time.Second)
            continue
        }
        
        // result[0] = "jobs:queue"
        // result[1] = job_id (actual job ID from Redis)
        jobId := result[1]
        
        // 3. Process job asynchronously (goroutine)
        go processJob(ctx, service, jobId)
    }
}
```

### Job Processing Flow
**File:** `/Users/s2s2hyun/Desktop/quel-canvas-server/modules/fashion/worker.go` (Lines 66-116)

```
processJob(jobID)
    ↓
FetchJobFromSupabase(jobID)  [Database Query #1]
    ↓
Switch on job.JobType:
    - "single_batch" → processSingleBatch()
    - "pipeline_stage" → processPipelineStage()
    - "simple_general" → processSimpleGeneral()
    - "simple_portrait" → processSimplePortrait()
```

---

## 2. DATABASE TABLES & COLUMNS QUERIED

### Table 1: `quel_production_jobs`
**Primary Query:**
```go
// File: modules/fashion/service.go (Lines 89-119)
func (s *Service) FetchJobFromSupabase(jobID string) (*model.ProductionJob, error) {
    data, _, err := s.supabase.From("quel_production_jobs").
        Select("*", "exact", false).
        Eq("job_id", jobID).
        Execute()
}
```

**Columns Accessed:**
| Column | Type | Usage |
|--------|------|-------|
| `job_id` | uuid | Query filter & identifier |
| `job_type` | varchar | Routes to processing function |
| `stage_index` | integer | Pipeline stage tracking |
| `job_status` | enum | Current status (pending→processing→completed) |
| `total_images` | integer | Target image count |
| `job_input_data` | jsonb | **CRITICAL - See below** |
| `production_id` | uuid | Links to production photo |
| `quel_member_id` | uuid | User ID |
| `org_id` | uuid | Organization ID (nullable) |
| `created_at` | timestamp | Job creation time |
| `started_at` | timestamp | Processing start |
| `completed_at` | timestamp | Processing end |
| `updated_at` | timestamp | Last update |

### Table 2: `quel_attach`
**Query for downloading images:**
```go
// File: modules/fashion/service.go (Lines 150-189)
func (s *Service) FetchAttachInfo(attachID int) (*model.Attach, error) {
    data, _, err := s.supabase.From("quel_attach").
        Select("*", "exact", false).
        Eq("attach_id", fmt.Sprintf("%d", attachID)).
        Execute()
}
```

**Columns Accessed:**
| Column | Type | Usage |
|--------|------|-------|
| `attach_id` | bigint | Primary key, image identifier |
| `attach_file_path` | varchar | Storage path (priority) |
| `attach_directory` | varchar | Fallback storage path |
| `attach_original_name` | varchar | Original filename |
| `attach_file_name` | varchar | Current filename |
| `attach_file_size` | bigint | File size in bytes |
| `attach_file_type` | varchar | MIME type (image/webp) |
| `attach_storage_type` | varchar | Storage service (supabase) |
| `created_at` | timestamp | Upload time |

**Insert Operation:**
```go
// File: modules/fashion/service.go (Lines 987-1034)
func (s *Service) CreateAttachRecord(ctx context.Context, filePath string, fileSize int64) (int, error) {
    insertData := map[string]interface{}{
        "attach_original_name": fileName,
        "attach_file_name":      fileName,
        "attach_file_path":      filePath,
        "attach_file_size":      fileSize,
        "attach_file_type":      "image/webp",
        "attach_directory":      filePath,
        "attach_storage_type":   "supabase",
    }
    data, _, err := s.supabase.From("quel_attach").
        Insert(insertData, false, "", "", "").
        Execute()
    // Returns newly created attach_id
}
```

### Table 3: `quel_production_photo`
**Update production status and attach IDs:**
```go
// File: modules/fashion/service.go (Lines 296-315)
func (s *Service) UpdateProductionPhotoStatus(ctx context.Context, productionID string, status string) error {
    updateData := map[string]interface{}{
        "production_status": status,
    }
    _, _, err := s.supabase.From("quel_production_photo").
        Update(updateData, "", "").
        Eq("production_id", productionID).
        Execute()
}

// File: modules/fashion/service.go (Lines 1074-1127)
func (s *Service) UpdateProductionAttachIds(ctx context.Context, productionID string, newAttachIds []int) error {
    // 1. SELECT existing attach_ids
    data, _, err := s.supabase.From("quel_production_photo").
        Select("attach_ids", "", false).
        Eq("production_id", productionID).
        Execute()
    
    // 2. MERGE new IDs into existing array
    // 3. UPDATE attach_ids column
    _, _, err = s.supabase.From("quel_production_photo").
        Update(updateData, "", "").
        Eq("production_id", productionID).
        Execute()
}
```

**Columns Accessed:**
| Column | Type | Usage |
|--------|------|-------|
| `production_id` | uuid | Identifier |
| `production_status` | enum | Status updates |
| `attach_ids` | jsonb | Array of generated image IDs |
| `production_name` | varchar | Metadata |
| `quel_member_id` | uuid | User reference |

### Table 4: `quel_production_jobs` (Updates)
**Progress tracking:**
```go
// File: modules/fashion/service.go (Lines 1036-1072)
func (s *Service) UpdateJobProgress(ctx context.Context, jobID string, completedImages int, generatedAttachIds []int) error {
    // Remove duplicates
    uniqueIds := make([]int, 0, len(generatedAttachIds))
    seen := make(map[int]bool)
    for _, id := range generatedAttachIds {
        if !seen[id] {
            seen[id] = true
            uniqueIds = append(uniqueIds, id)
        }
    }
    
    updateData := map[string]interface{}{
        "completed_images":     completedImages,
        "generated_attach_ids": uniqueIds,
        "updated_at":           "now()",
    }
    _, _, err := s.supabase.From("quel_production_jobs").
        Update(updateData, "", "").
        Eq("job_id", jobID).
        Execute()
}
```

**Columns Updated:**
- `completed_images`: Current progress count
- `generated_attach_ids`: JSONB array of created image IDs
- `updated_at`: Last modification timestamp

### Table 5: `quel_organization` (Credits)
**For credit deduction:**
```go
// File: modules/fashion/service.go (Lines 1129-onwards)
func (s *Service) DeductCredits(ctx context.Context, userID string, orgID *string, productionID string, attachIds []int) error {
    // If organization credits:
    data, _, err := s.supabase.From("quel_organization").
        Select("org_credit", "", false).
        Eq("org_id", *orgID).
        Execute()
    
    // Update org_credit
    _, _, err = s.supabase.From("quel_organization").
        Update(map[string]interface{}{
            "org_credit": newBalance,
        }, "", "").
        Eq("org_id", *orgID).
        Execute()
}
```

**Columns Accessed:**
- `org_credit`: Organization credit balance (SELECT & UPDATE)
- `org_id`: Organization identifier

---

## 3. JOB INPUT DATA STRUCTURE (JSONB)

### Location in Job Object
```
quel_production_jobs.job_input_data (JSONB column)
```

### Stage 0/1 Pipeline Mode Example:
```json
{
  "basePrompt": "best quality, masterpiece",
  "stages": [
    {
      "stage_index": 0,
      "prompt": "Stage 0 prompt text",
      "quantity": 2,
      "aspect-ratio": "16:9",
      "individualImageAttachIds": [
        {
          "attachId": 39070,
          "type": "model"
        },
        {
          "attachId": 39071,
          "type": "top"
        }
      ]
    },
    {
      "stage_index": 1,
      "prompt": "Stage 1 prompt text",
      "quantity": 2,
      "aspect-ratio": "16:9",
      "individualImageAttachIds": [
        {
          "attachId": 39072,
          "type": "model"
        }
      ]
    }
  ],
  "userId": "user-uuid-here"
}
```

### Single Batch Mode Example:
```json
{
  "basePrompt": "best quality, masterpiece",
  "individualImageAttachIds": [
    {
      "attachId": 39070,
      "type": "model"
    },
    {
      "attachId": 39071,
      "type": "top"
    }
  ],
  "combinations": [
    {
      "angle": "front",
      "shot": "full",
      "quantity": 2
    },
    {
      "angle": "side",
      "shot": "full",
      "quantity": 1
    }
  ],
  "aspect-ratio": "16:9",
  "userId": "user-uuid-here"
}
```

### Data Extraction in Worker:
```go
// File: modules/fashion/worker.go (Lines 118-206)
func processSingleBatch(ctx context.Context, service *Service, job *model.ProductionJob) {
    // Extract individualImageAttachIds
    individualImageAttachIds, ok := job.JobInputData["individualImageAttachIds"].([]interface{})
    
    // Extract basePrompt
    basePrompt := fallback.SafeString(job.JobInputData["basePrompt"], "best quality, masterpiece")
    
    // Extract combinations
    combinations := fallback.NormalizeCombinations(
        job.JobInputData["combinations"],
        fallback.DefaultQuantity(job.TotalImages),
        "front",
        "full",
    )
    
    // Extract aspect-ratio
    aspectRatio := fallback.SafeAspectRatio(job.JobInputData["aspect-ratio"])
    
    // Extract userId
    userID := fallback.SafeString(job.JobInputData["userId"], "")
}

// Pipeline stage extraction (Lines 481-560)
// For each stage in stages array:
stages, ok := job.JobInputData["stages"].([]interface{})
for stageIdx, stageData := range stages {
    stage, ok := stageData.(map[string]interface{})
    
    stageIndex := getIntFromInterface(stage["stage_index"], idx)
    prompt := fallback.SafeString(stage["prompt"], defaultPrompt)
    quantity := getIntFromInterface(stage["quantity"], 1)
    aspectRatio := fallback.SafeAspectRatio(stage["aspect-ratio"])
    
    // Extract individual image attachments for this stage
    if individualIds, ok := stage["individualImageAttachIds"].([]interface{}); ok {
        for i, attachObj := range individualIds {
            attachMap, ok := attachObj.(map[string]interface{})
            attachIDFloat, ok := attachMap["attachId"].(float64)
            attachID := int(attachIDFloat)
            attachType, _ := attachMap["type"].(string)
            // Download and categorize
        }
    }
}
```

---

## 4. IMAGE CATEGORIZATION & ATTACHMENT HANDLING

### Attachment ID Processing Pattern:
```go
// Lines 158-206: Processing individualImageAttachIds
for i, attachObj := range individualImageAttachIds {
    attachMap, ok := attachObj.(map[string]interface{})
    if !ok {
        log.Printf("Invalid attach object at index %d", i)
        continue
    }
    
    // Extract attachment ID (comes as float64 from JSON)
    attachIDFloat, ok := attachMap["attachId"].(float64)
    if !ok {
        log.Printf("Failed to get attachId at index %d", i)
        continue
    }
    
    attachID := int(attachIDFloat)  // Convert to int
    attachType, _ := attachMap["type"].(string)
    
    log.Printf("Downloading image %d/%d: AttachID=%d, Type=%s",
        i+1, len(individualImageAttachIds), attachID, attachType)
    
    // FETCH FROM quel_attach (by attachID)
    imageData, err := service.DownloadImageFromStorage(attachID)
    if err != nil {
        log.Printf("Failed to download image %d: %v", attachID, err)
        continue
    }
    
    // Categorize by type
    switch attachType {
    case "model":
        categories.Model = imageData
    case "background", "bg":
        categories.Background = imageData
    default:
        if clothingTypes[attachType] {  // top, pants, outer
            categories.Clothing = append(categories.Clothing, imageData)
        } else if accessoryTypes[attachType] {  // shoes, bag, accessory, acce
            categories.Accessories = append(categories.Accessories, imageData)
        }
    }
}
```

### Image Categories Struct:
```go
// File: modules/fashion/service.go (Lines 38-44)
type ImageCategories struct {
    Model       []byte   // Model image (max 1)
    Clothing    [][]byte // Clothing images array (top, pants, outer)
    Accessories [][]byte // Accessory images array (shoes, bag, accessory)
    Background  []byte   // Background image (max 1)
}
```

### Image Download Process:
```go
// File: modules/fashion/service.go (Lines 191-250)
func (s *Service) DownloadImageFromStorage(attachID int) ([]byte, error) {
    // 1. Query quel_attach table
    attach, err := s.FetchAttachInfo(attachID)
    if err != nil {
        return nil, err
    }
    
    // 2. Check attach_file_path (priority) or attach_directory (fallback)
    var filePath string
    if attach.AttachFilePath != nil && *attach.AttachFilePath != "" {
        filePath = *attach.AttachFilePath
        log.Printf("🔍 Using attach_file_path: %s", filePath)
    } else if attach.AttachDirectory != nil && *attach.AttachDirectory != "" {
        filePath = *attach.AttachDirectory
        log.Printf("🔍 Using attach_directory: %s", filePath)
    } else {
        return nil, fmt.Errorf("no file path found for attach_id: %d", attachID)
    }
    
    // 3. Fix path if needed (add uploads/ prefix)
    if len(filePath) > 0 && filePath[0] != '/' &&
        len(filePath) >= 7 && filePath[:7] == "upload-" {
        filePath = "uploads/" + filePath
    }
    
    // 4. Download from Supabase Storage
    fullURL := cfg.SupabaseStorageBaseURL + filePath
    httpResp, err := http.Get(fullURL)
    if err != nil {
        return nil, err
    }
    defer httpResp.Body.Close()
    
    imageData, err := io.ReadAll(httpResp.Body)
    return imageData, nil
}
```

---

## 5. DATABASE QUERY SEQUENCE FOR SINGLE JOB

### Complete Job Processing Cycle:

```
1. FETCH JOB (Query #1)
   ==================
   SELECT * FROM quel_production_jobs
   WHERE job_id = 'job-uuid'
   
   Returns: ProductionJob object with:
   - job_id, job_type, job_input_data (stages/combinations)
   - production_id, total_images, job_status
   - quel_member_id, org_id

2. UPDATE JOB STATUS TO PROCESSING (Query #2)
   ==========================================
   UPDATE quel_production_jobs
   SET job_status = 'processing',
       started_at = now(),
       updated_at = now()
   WHERE job_id = 'job-uuid'

3. UPDATE PRODUCTION STATUS TO PROCESSING (Query #3)
   ================================================
   UPDATE quel_production_photo
   SET production_status = 'processing'
   WHERE production_id = 'prod-uuid'

4. FOR EACH STAGE / COMBINATION:
   
   A. FETCH ATTACH INFO (Query #4, #5, #6... for each image)
      ======================================================
      SELECT * FROM quel_attach
      WHERE attach_id = 39070  -- from individualImageAttachIds
      
      Returns: attach_file_path, attach_directory
      
      Download image from: 
      GET ${SupabaseStorageBaseURL}${attach_file_path}

   B. GENERATE IMAGE WITH GEMINI API (external API, not DB)
      ====================================================
      Call: Gemini generateContent()
      Input: Categories (categorized images) + Prompt
      Output: Base64 encoded PNG image

   C. CONVERT AND UPLOAD WEBP (Query #7, #8, #9...)
      ============================================
      POST to Supabase Storage (external, not DB query)
      Returns: filePath

   D. CREATE ATTACH RECORD (Query varies)
      ==================================
      INSERT INTO quel_attach (
          attach_original_name,
          attach_file_name,
          attach_file_path,
          attach_file_size,
          attach_file_type,
          attach_directory,
          attach_storage_type,
          created_at
      ) VALUES (...)
      
      Returns: newly created attach_id (e.g., 39100)

   E. DEDUCT CREDITS (Query #N)
      =======================
      If org_id present:
      
      SELECT org_credit FROM quel_organization
      WHERE org_id = 'org-uuid'
      
      UPDATE quel_organization
      SET org_credit = org_credit - credits_cost
      WHERE org_id = 'org-uuid'
      
      INSERT INTO quel_credits (
          user_id, transaction_type='deduction',
          amount, balance_after, attach_idx,
          production_idx, description
      ) VALUES (...)

   F. UPDATE JOB PROGRESS (Query #N+1)
      ================================
      UPDATE quel_production_jobs
      SET completed_images = current_count,
          generated_attach_ids = [39100, 39101, ...],
          updated_at = now()
      WHERE job_id = 'job-uuid'

5. UPDATE PRODUCTION ATTACH IDS (Query #N+2)
   ========================================
   SELECT attach_ids FROM quel_production_photo
   WHERE production_id = 'prod-uuid'
   
   -- Merge existing IDs with new IDs
   
   UPDATE quel_production_photo
   SET attach_ids = [39100, 39101, 39102, ...]
   WHERE production_id = 'prod-uuid'

6. UPDATE FINAL JOB STATUS (Query #N+3)
   ===================================
   UPDATE quel_production_jobs
   SET job_status = 'completed',
       completed_at = now(),
       updated_at = now()
   WHERE job_id = 'job-uuid'

7. UPDATE FINAL PRODUCTION STATUS (Query #N+4)
   =========================================
   UPDATE quel_production_photo
   SET production_status = 'completed'
   WHERE production_id = 'prod-uuid'
```

---

## 6. CRITICAL COLUMNS FOR STAGE 0/1 PROCESSING

### Stage 0/1 Prompts Handling:
```go
// File: modules/fashion/worker.go (Lines 481-560)

// Extract stages array from job_input_data
stages, ok := job.JobInputData["stages"].([]interface{})
if !ok || len(stages) == 0 {
    // Create default single stage
    stages = []interface{}{
        map[string]interface{}{
            "stage_index": 0,
            "prompt":      defaultPrompt,  // "Stage 0 prompt"
            "quantity":    fallback.DefaultQuantity(job.TotalImages),
        },
    }
}

for stageIdx, stageData := range stages {
    stage, ok := stageData.(map[string]interface{})
    
    // Extract Stage 0/1 specific data
    stageIndex := getIntFromInterface(stage["stage_index"], idx)
    prompt := fallback.SafeString(stage["prompt"], defaultPrompt)
    quantity := getIntFromInterface(stage["quantity"], 1)
    aspectRatio := fallback.SafeAspectRatio(stage["aspect-ratio"])
    
    log.Printf("🎬 Stage %d/%d: Processing %d images with aspect-ratio %s",
        stageIndex+1, len(stages), quantity, aspectRatio)
    
    // Extract individual image attachments for Stage N
    if individualIds, ok := stage["individualImageAttachIds"].([]interface{}); ok {
        // Process images specific to this stage
        for i, attachObj := range individualIds {
            attachMap, ok := attachObj.(map[string]interface{})
            attachID := int(attachMap["attachId"].(float64))
            attachType, _ := attachMap["type"].(string)
            
            // Download from quel_attach using this stage's attachID
            imageData, err := service.DownloadImageFromStorage(attachID)
        }
    }
}
```

---

## 7. KEY MODELS & DATA TYPES

### Model Definition:
```go
// File: modules/common/model/model.go (Lines 5-74)

type ProductionJob struct {
    JobID              string                 `json:"job_id"`
    ProductionID       *string                `json:"production_id"`
    QuelProductionPath string                 `json:"quel_production_path"` // Category
    JobType            string                 `json:"job_type"`
    StageIndex         *int                   `json:"stage_index"`
    StageName          *string                `json:"stage_name"`
    BatchIndex         *int                   `json:"batch_index"`
    JobStatus          string                 `json:"job_status"`
    TotalImages        int                    `json:"total_images"`
    CompletedImages    int                    `json:"completed_images"`
    FailedImages       int                    `json:"failed_images"`
    JobInputData       map[string]interface{} `json:"job_input_data"` // CRITICAL: Contains prompts & image IDs
    GeneratedAttachIDs []interface{}          `json:"generated_attach_ids"`
    ErrorMessage       *string                `json:"error_message"`
    RetryCount         int                    `json:"retry_count"`
    CreatedAt          time.Time              `json:"created_at"`
    StartedAt          *time.Time             `json:"started_at"`
    CompletedAt        *time.Time             `json:"completed_at"`
    UpdatedAt          time.Time              `json:"updated_at"`
    QuelMemberID       *string                `json:"quel_member_id"`
    OrgID              *string                `json:"org_id"`
    EstimatedCredits   int                    `json:"estimated_credits"`
}

type Attach struct {
    AttachID           int64     `json:"attach_id"`
    CreatedAt          time.Time `json:"created_at"`
    AttachOriginalName *string   `json:"attach_original_name"`
    AttachFileName     *string   `json:"attach_file_name"`
    AttachFilePath     *string   `json:"attach_file_path"`  // Priority path
    AttachFileSize     *int64    `json:"attach_file_size"`
    AttachFileType     *string   `json:"attach_file_type"`
    AttachDirectory    *string   `json:"attach_directory"` // Fallback path
    AttachStorageType  *string   `json:"attach_storage_type"` // "supabase"
}
```

---

## 8. SUMMARY OF DATABASE TABLES & COLUMNS

| Table | Key Columns | Operation | Purpose |
|-------|------------|-----------|---------|
| **quel_production_jobs** | job_id, job_type, job_input_data, job_status, generated_attach_ids, completed_images, production_id, org_id | SELECT, UPDATE | Job metadata, progress tracking, pipeline config |
| **quel_production_photo** | production_id, production_status, attach_ids | SELECT, UPDATE | Production metadata, generated image list |
| **quel_attach** | attach_id, attach_file_path, attach_directory, attach_file_size, attach_file_type | SELECT, INSERT | Image metadata, storage paths, retrieval |
| **quel_organization** | org_id, org_credit | SELECT, UPDATE | Organization credit balance |
| **quel_credits** | user_id, transaction_type, amount, attach_idx | INSERT | Credit transaction logging |

---

## 9. LOG PATTERNS TO TRACE

When reviewing logs for job processing:

```
1. Redis Queue Entry:
   "👀 Watching queue: jobs:queue"
   "Received new job: <job_id>"

2. Job Fetching:
   "🔍 Fetching job from Supabase: <job_id>"
   "✅ Job fetched successfully"

3. Processing Mode Detection:
   "Single Batch Mode - Processing X images"
   "Pipeline Stage Mode - Processing stage Y"

4. Stage 0/1 Prompts:
   "🎬 Stage 0/1: Processing X images with aspect-ratio"
   "Stage N: Using individualImageAttachIds (X images)"

5. Image Attachment Downloads:
   "Fetching attach info: 39070" (image ID from logs)
   "Using attach_file_path: <path>"
   "Image downloaded successfully: X bytes"

6. Categorization:
   "Model image added"
   "Clothing image added (type: top)"
   "Accessory image added (type: shoes)"

7. Generation & Upload:
   "Calling Gemini API (model: X) with prompt length: Y"
   "Image X/Y completed: AttachID=39100"
   "Attach record created: ID=39100"

8. Progress Tracking:
   "📊 Updating job progress: X/Y completed"
   "Merged attach_ids: X existing + Y new = Z total"

9. Completion:
   "🎬 Stage N completed: X/Y images generated"
   "Job <job_id> finished: X/Y images completed"
```

---

## 10. COMPLETE QUERY EXECUTION ORDER

For a single job with 2 stages and 2 images each:

```
QUERY 1: SELECT * FROM quel_production_jobs WHERE job_id = 'xyz'
QUERY 2: UPDATE quel_production_jobs SET job_status='processing' WHERE job_id = 'xyz'
QUERY 3: UPDATE quel_production_photo SET production_status='processing' WHERE production_id = 'abc'

[For Stage 0, Image 1]:
QUERY 4: SELECT * FROM quel_attach WHERE attach_id = 39070
QUERY 5: INSERT INTO quel_attach (...) VALUES (...) RETURNING attach_id=39100
QUERY 6: SELECT org_credit FROM quel_organization WHERE org_id = 'org-xyz'
QUERY 7: UPDATE quel_organization SET org_credit=... WHERE org_id = 'org-xyz'
QUERY 8: INSERT INTO quel_credits (...) VALUES (...)
QUERY 9: UPDATE quel_production_jobs SET completed_images=1, generated_attach_ids=[39100]

[For Stage 0, Image 2]:
QUERY 10: SELECT * FROM quel_attach WHERE attach_id = 39071
QUERY 11: INSERT INTO quel_attach (...) VALUES (...) RETURNING attach_id=39101
QUERY 12: SELECT org_credit FROM quel_organization WHERE org_id = 'org-xyz'
QUERY 13: UPDATE quel_organization SET org_credit=... WHERE org_id = 'org-xyz'
QUERY 14: INSERT INTO quel_credits (...) VALUES (...)
QUERY 15: UPDATE quel_production_jobs SET completed_images=2, generated_attach_ids=[39100,39101]

[For Stage 1, Image 1]:
QUERY 16: SELECT * FROM quel_attach WHERE attach_id = 39072
... (similar sequence)

[For Stage 1, Image 2]:
... (similar sequence)

[Final Updates]:
QUERY N-3: SELECT attach_ids FROM quel_production_photo WHERE production_id = 'abc'
QUERY N-2: UPDATE quel_production_photo SET attach_ids=[39100,39101,39102,39103]
QUERY N-1: UPDATE quel_production_jobs SET job_status='completed', completed_at=now()
QUERY N: UPDATE quel_production_photo SET production_status='completed'
```

