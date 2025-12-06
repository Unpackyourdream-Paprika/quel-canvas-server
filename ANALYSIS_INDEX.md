# Redis Job Processing Analysis - Documentation Index

This directory now contains comprehensive documentation of how Redis jobs are processed and what database tables/columns are used.

## Documents Created

### 1. **REDIS_JOB_PROCESSING_ANALYSIS.md** (25 KB)
Complete technical analysis covering:
- Redis job consumer/processor code (worker.go StartWorker function)
- Full database table & column specifications
- Job input data structure (JSONB) for Stage 0/1 pipelines
- Image categorization and attachment handling
- Complete database query sequences
- Critical columns for Stage 0/1 processing
- Key data models and types
- Log patterns for tracing execution

**Best for:** Deep technical understanding, tracing specific queries, understanding complete flow

### 2. **DATABASE_COLUMNS_REFERENCE.md** (10 KB)
Detailed SQL reference including:
- All quel_production_jobs columns with types and usage
- Complete job_input_data JSONB structure (Pipeline & Single Batch modes)
- quel_attach table with file path resolution logic
- quel_production_photo structure and operations
- quel_organization and quel_credits tables
- Image type categories (model, top, shoes, bag, etc.)
- Complete query flow example with actual SQL
- Critical columns summary table

**Best for:** SQL queries, understanding data structures, debugging specific table operations

### 3. **QUICK_REFERENCE.md** (5 KB)
Quick lookup guide covering:
- Worker file locations
- Redis queue operations
- Critical data tables summary
- Stage 0/1 pipeline processing
- Image attachment ID processing
- Database query sequence overview
- Key log patterns
- Important implementation notes

**Best for:** Quick lookups, understanding at a glance, reference while debugging

---

## Key Findings Summary

### Redis Queue
- **Queue Name:** `jobs:queue`
- **Operation:** `BRPOP` (Blocking Right Pop)
- **Entry Point:** `modules/fashion/worker.go:20` - StartWorker()
- **Processing:** Asynchronous goroutine per job

### Critical Database Operations

1. **Fetch Job:** `SELECT * FROM quel_production_jobs WHERE job_id = ?`
   - Returns: job_input_data (JSONB) with stages/combinations
   - Key fields: job_type, total_images, job_status

2. **Fetch Image Metadata:** `SELECT * FROM quel_attach WHERE attach_id = 39070`
   - Returns: attach_file_path (priority) or attach_directory (fallback)
   - Used to: Download original images from Supabase Storage

3. **Create Generated Image:** `INSERT INTO quel_attach (...)`
   - Creates record for newly generated image
   - Returns: newly created attach_id (e.g., 39100)

4. **Update Progress:** `UPDATE quel_production_jobs SET completed_images = ?, generated_attach_ids = ?`
   - Tracks progress in real-time
   - generated_attach_ids is JSONB array: [39100, 39101, ...]

5. **Deduct Credits:** `SELECT/UPDATE quel_organization SET org_credit = org_credit - 20`
   - If org_id present, deduct from organization credits
   - Otherwise, deduct from user credits

### Stage 0/1 Pipeline Processing

```
job_input_data.stages = [
  {
    stage_index: 0,
    prompt: "Stage 0 prompt",
    quantity: 2,
    individualImageAttachIds: [
      { attachId: 39070, type: "model" },
      { attachId: 39071, type: "top" }
    ]
  },
  {
    stage_index: 1,
    prompt: "Stage 1 prompt",
    ...
  }
]
```

### Image Attachment IDs

- **Input IDs:** `job_input_data.individualImageAttachIds` (e.g., 39070)
  - Come as float64 from JSON, convert to int
  - Each has a type: "model", "top", "shoes", "bag", etc.

- **Output IDs:** `quel_production_jobs.generated_attach_ids` (e.g., 39100)
  - JSONB array of newly created image IDs
  - Merged into `quel_production_photo.attach_ids`

---

## Quick Navigation

### To understand:
- **"How are jobs picked up from Redis?"** → REDIS_JOB_PROCESSING_ANALYSIS.md, Section 1
- **"What columns does quel_production_jobs have?"** → DATABASE_COLUMNS_REFERENCE.md, Table: quel_production_jobs
- **"How are Stage 0/1 prompts handled?"** → REDIS_JOB_PROCESSING_ANALYSIS.md, Section 6
- **"What database queries happen during job processing?"** → DATABASE_COLUMNS_REFERENCE.md, Complete Query Flow Example
- **"Where is the worker code?"** → QUICK_REFERENCE.md, File Paths in Code
- **"What's the job_input_data JSONB structure?"** → DATABASE_COLUMNS_REFERENCE.md, job_input_data Structure

---

## File Locations in Code

### Worker Entry Points
```
/modules/fashion/worker.go:20     - StartWorker()
/modules/fashion/worker.go:66     - processJob()
/modules/fashion/worker.go:118    - processSingleBatch()
/modules/fashion/worker.go:481    - processPipelineStage()
```

### Service Functions
```
/modules/fashion/service.go:89    - FetchJobFromSupabase()
/modules/fashion/service.go:150   - FetchAttachInfo()
/modules/fashion/service.go:191   - DownloadImageFromStorage()
/modules/fashion/service.go:987   - CreateAttachRecord()
/modules/fashion/service.go:1036  - UpdateJobProgress()
/modules/fashion/service.go:1074  - UpdateProductionAttachIds()
```

### Models
```
/modules/common/model/model.go:5   - ProductionJob struct
/modules/common/model/model.go:54  - Attach struct
```

---

## Tables Quick Reference

| Table | Purpose | Key Columns |
|-------|---------|------------|
| `quel_production_jobs` | Job configuration & progress | job_id, job_type, job_input_data, generated_attach_ids |
| `quel_attach` | Image metadata & storage paths | attach_id, attach_file_path, attach_directory |
| `quel_production_photo` | Production tracking | production_id, production_status, attach_ids |
| `quel_organization` | Organization credits | org_id, org_credit |
| `quel_credits` | Transaction logging | user_id, transaction_type, amount, attach_idx |

---

## Log Pattern Examples

When tracing job processing, look for:
```
"Received new job: <job-id>"
"🎬 Stage 0/1: Processing X images"
"Downloading image: AttachID=39070"
"Fetching attach info: 39070"
"Image downloaded successfully: X bytes"
"Calling Gemini API"
"Attach record created: ID=39100"
"📊 Updating job progress: X/Y completed"
"Stage N completed"
```

---

## Key Implementation Details

1. **job_input_data is JSONB** - Contains all job configuration
2. **Attachment IDs come as float64** - Must convert to int
3. **attach_file_path has priority** - Falls back to attach_directory
4. **generated_attach_ids is array** - Tracked for progress
5. **All modules follow same pattern** - Fashion, beauty, cinema, cartoon, eats
6. **Supabase serves dual purpose** - Database AND file storage
7. **Parallel processing** - Stages/combinations use goroutines with semaphore (max 2 concurrent)

---

## Related Documentation

- Database table schemas: `/Users/s2s2hyun/Desktop/quel-canvas-server/.claude/database-tables/`
  - `quel_production_jobs.md`
  - `quel_production_photo.md`
  - `TABLE_STRUCTURES.md`

---

**Last Updated:** 2025-12-06
**Analysis Scope:** Redis job processing, database queries, Stage 0/1 pipeline handling
**Modules Analyzed:** Fashion module (pattern applies to all: beauty, cinema, cartoon, eats)
