# Redis Job Processing - Quick Reference

## Worker Files Location
- **Fashion Module:** `/modules/fashion/worker.go` (Lines 20-65: StartWorker)
- **Fashion Service:** `/modules/fashion/service.go` (Database operations)
- **Other Modules:** cinema, beauty, cartoon, eats, generate-image (same pattern)

## Redis Queue Operations
```
Queue Name: "jobs:queue"
Operation: BRPOP (Blocking Right Pop)
Job Retrieval: go processJob(ctx, service, jobId)
```

## Critical Data Tables

### 1. quel_production_jobs (Job Metadata)
```
SELECT * FROM quel_production_jobs WHERE job_id = ?

Key Columns:
- job_input_data (JSONB) -> Contains: stages, prompt, individualImageAttachIds
- job_type -> "pipeline_stage", "single_batch", etc.
- generated_attach_ids -> Array of created image IDs
- job_status -> pending, processing, completed, failed
```

### 2. quel_attach (Image Metadata & Storage Paths)
```
SELECT * FROM quel_attach WHERE attach_id = ?

Key Columns (for retrieval):
- attach_file_path (PRIORITY) or attach_directory (fallback)
- attach_id -> Image identifier (e.g., 39070)
- attach_file_type -> "image/webp"

INSERT INTO quel_attach when creating new images
-> Returns newly created attach_id (e.g., 39100)
```

### 3. quel_production_photo (Production Tracking)
```
UPDATE production_status = 'processing|completed'
SELECT/UPDATE attach_ids (JSONB array)
```

### 4. quel_organization (Credit Deduction)
```
SELECT/UPDATE org_credit for organization credit tracking
```

## Stage 0/1 Pipeline Processing

### How Stages Work:
```
job_input_data.stages = [
  {
    stage_index: 0,
    prompt: "Stage 0 prompt text",
    quantity: 2,
    individualImageAttachIds: [
      { attachId: 39070, type: "model" },
      { attachId: 39071, type: "top" }
    ]
  },
  {
    stage_index: 1,
    prompt: "Stage 1 prompt text",
    quantity: 2,
    individualImageAttachIds: [...]
  }
]
```

### Processing Flow for Each Stage:
1. Extract stage data from job_input_data.stages[N]
2. For each image in individualImageAttachIds:
   - Query quel_attach with attachId
   - Download from Supabase Storage
   - Categorize by type (model, top, shoes, etc.)
3. Call Gemini API with categorized images + stage prompt
4. Create new attach record (INSERT quel_attach)
5. Update job progress

## Image Attachment IDs

### Where They Appear:
- **Input IDs:** job_input_data.individualImageAttachIds (like 39070)
- **Output IDs:** generated_attach_ids in quel_production_jobs (like 39100)

### ID Processing:
```
individualImageAttachIds[0] -> {
  attachId: 39070 (float64 from JSON),
  type: "model|top|shoes|bag|etc"
}

Convert: int(attachIDFloat) -> attachID = 39070

Query quel_attach(39070) for:
- attach_file_path -> Download from Supabase
- Returns image bytes -> Process -> Generate new image
- Create quel_attach record -> Returns 39100
```

## Database Query Sequence

For a single 2-stage job with 2 images per stage (4 total):

```
1. FETCH JOB: SELECT * FROM quel_production_jobs WHERE job_id
2. UPDATE JOB STATUS: to 'processing'
3. UPDATE PRODUCTION: status to 'processing'

[Repeat for each image: 4x]
4. FETCH ATTACH: SELECT * FROM quel_attach WHERE attach_id = 39070
5. INSERT NEW ATTACH: CREATE attach record -> Returns 39100
6. SELECT ORG CREDIT: (if org_id present)
7. UPDATE ORG CREDIT: Deduct credits
8. INSERT CREDIT LOG: Transaction record
9. UPDATE JOB PROGRESS: Update completed_images, generated_attach_ids

[After all images]
10. SELECT PRODUCTION ATTACH_IDS: Get existing IDs
11. UPDATE PRODUCTION: Merge new IDs into attach_ids array
12. UPDATE JOB FINAL: status='completed', completed_at=now()
13. UPDATE PRODUCTION FINAL: status='completed'
```

## Key Log Patterns

```
"Received new job: <job_id>"
"🎬 Stage 0/1: Processing X images"
"Downloading image: AttachID=39070, Type=model"
"Fetching attach info: 39070"
"Image downloaded successfully: X bytes"
"Calling Gemini API"
"Attach record created: ID=39100"
"Updating job progress: X/Y completed"
"Stage N completed: X/Y images generated"
```

## File Paths in Code

### Worker Entry Points:
- `modules/fashion/worker.go:20` - StartWorker()
- `modules/fashion/worker.go:66` - processJob()
- `modules/fashion/worker.go:118` - processSingleBatch()
- `modules/fashion/worker.go:481` - processPipelineStage()

### Service Functions:
- `modules/fashion/service.go:89` - FetchJobFromSupabase()
- `modules/fashion/service.go:150` - FetchAttachInfo()
- `modules/fashion/service.go:191` - DownloadImageFromStorage()
- `modules/fashion/service.go:987` - CreateAttachRecord()
- `modules/fashion/service.go:1036` - UpdateJobProgress()
- `modules/fashion/service.go:1074` - UpdateProductionAttachIds()

### Models:
- `modules/common/model/model.go:5` - ProductionJob
- `modules/common/model/model.go:54` - Attach

## Important Notes

1. **job_input_data is JSONB**: Contains all configuration (prompts, image IDs, combinations)
2. **Attachment IDs come as float64**: Must convert to int before use
3. **attach_file_path has priority**: Falls back to attach_directory
4. **generated_attach_ids is array**: Tracked in quel_production_jobs for progress
5. **All modules use same pattern**: Fashion, beauty, cinema, cartoon, eats
6. **Supabase is both**: Database AND file storage
7. **Parallel processing**: Stages/combinations processed with goroutines

