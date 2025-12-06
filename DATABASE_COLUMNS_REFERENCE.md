# Database Columns Reference - Job Processing

## Table: quel_production_jobs

### SELECT Operations (Fetch Job Data)
```sql
SELECT 
  job_id,                    -- UUID: Job identifier
  job_type,                  -- VARCHAR: "single_batch", "pipeline_stage", "simple_general", "simple_portrait"
  job_input_data,            -- JSONB: CRITICAL - Contains all configuration
  production_id,             -- UUID: Links to quel_production_photo
  job_status,                -- ENUM: pending, processing, completed, failed
  total_images,              -- INT: Target image count
  completed_images,          -- INT: Progress counter
  failed_images,             -- INT: Failed count
  generated_attach_ids,      -- JSONB: Array of created image IDs [39100, 39101, ...]
  quel_member_id,            -- UUID: User who created job
  org_id,                    -- UUID (nullable): Organization for credit deduction
  stage_index,               -- INT (nullable): For pipeline_stage mode
  created_at,                -- TIMESTAMP: Job creation
  started_at,                -- TIMESTAMP: When processing started
  completed_at,              -- TIMESTAMP: When processing ended
  updated_at                 -- TIMESTAMP: Last modification
FROM quel_production_jobs 
WHERE job_id = '...'
```

### job_input_data Structure (JSONB)

#### Pipeline Stage Mode:
```json
{
  "basePrompt": "best quality, masterpiece",
  "stages": [
    {
      "stage_index": 0,
      "prompt": "Stage 0 specific prompt text",
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
      "prompt": "Stage 1 specific prompt text",
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
  "userId": "member-uuid"
}
```

#### Single Batch Mode:
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
    }
  ],
  "aspect-ratio": "16:9",
  "userId": "member-uuid"
}
```

### UPDATE Operations (Progress Tracking)
```sql
UPDATE quel_production_jobs
SET
  job_status = 'processing',              -- When job starts
  started_at = NOW(),
  updated_at = NOW()
WHERE job_id = '...'

-- During processing (repeated for each image)
UPDATE quel_production_jobs
SET
  completed_images = 1,                   -- Increment counter
  generated_attach_ids = '[39100]'::jsonb,  -- Add new image ID
  updated_at = NOW()
WHERE job_id = '...'

-- When finished
UPDATE quel_production_jobs
SET
  job_status = 'completed',
  completed_at = NOW(),
  updated_at = NOW()
WHERE job_id = '...'
```

---

## Table: quel_attach

### SELECT Operations (Fetch Image Metadata)
```sql
SELECT 
  attach_id,                 -- BIGINT: Image identifier (PRIMARY KEY)
  attach_file_path,          -- VARCHAR (PRIORITY): Path to file in storage
  attach_directory,          -- VARCHAR (FALLBACK): Alternative path
  attach_original_name,      -- VARCHAR: Original filename
  attach_file_name,          -- VARCHAR: Current filename
  attach_file_size,          -- BIGINT: File size in bytes
  attach_file_type,          -- VARCHAR: MIME type (e.g., "image/webp")
  attach_storage_type,       -- VARCHAR: Storage service ("supabase")
  created_at                 -- TIMESTAMP: Upload time
FROM quel_attach 
WHERE attach_id = 39070
```

### File Path Resolution
```
IF attach_file_path IS NOT NULL AND attach_file_path != ''
  THEN use attach_file_path
ELSE IF attach_directory IS NOT NULL AND attach_directory != ''
  THEN use attach_directory
ELSE
  ERROR: "no file path found"
```

### Path Example
```
attach_file_path: "uploads/generated/fashion/abc-def-123.webp"
Download from: ${SupabaseStorageBaseURL}${attach_file_path}
```

### INSERT Operations (Create New Image Record)
```sql
INSERT INTO quel_attach (
  attach_original_name,     -- VARCHAR: e.g., "123.webp"
  attach_file_name,         -- VARCHAR: e.g., "123.webp"
  attach_file_path,         -- VARCHAR: e.g., "uploads/generated/fashion/123.webp"
  attach_file_size,         -- BIGINT: File size in bytes
  attach_file_type,         -- VARCHAR: "image/webp"
  attach_directory,         -- VARCHAR: Same as attach_file_path
  attach_storage_type,      -- VARCHAR: "supabase"
  created_at                -- TIMESTAMP: NOW()
) VALUES (...)
RETURNING attach_id         -- Returns newly created ID (e.g., 39100)
```

---

## Table: quel_production_photo

### SELECT Operations
```sql
SELECT 
  production_id,             -- UUID: Production identifier
  attach_ids,                -- JSONB: Array of generated image IDs [39100, 39101, ...]
  production_status,         -- ENUM: pending, processing, completed, failed
  quel_member_id,            -- UUID: User/member
  production_name,           -- VARCHAR: Production name
  created_at                 -- TIMESTAMP: Creation time
FROM quel_production_photo 
WHERE production_id = '...'
```

### UPDATE Operations
```sql
-- Update status to processing
UPDATE quel_production_photo
SET production_status = 'processing'
WHERE production_id = '...'

-- Merge new attach IDs with existing ones
UPDATE quel_production_photo
SET attach_ids = '[39070, 39071, 39100, 39101]'::jsonb
WHERE production_id = '...'

-- Update final status
UPDATE quel_production_photo
SET production_status = 'completed'
WHERE production_id = '...'
```

---

## Table: quel_organization

### SELECT Operations (Credit Check)
```sql
SELECT 
  org_id,                    -- UUID: Organization identifier
  org_credit                 -- INT: Credit balance
FROM quel_organization 
WHERE org_id = '...'
```

### UPDATE Operations (Credit Deduction)
```sql
UPDATE quel_organization
SET org_credit = org_credit - 20  -- Deduct credits per image
WHERE org_id = '...'
```

---

## Table: quel_credits

### INSERT Operations (Transaction Log)
```sql
INSERT INTO quel_credits (
  user_id,                   -- UUID: Member ID
  transaction_type,          -- VARCHAR: 'deduction' or 'purchase'
  amount,                    -- INT: Credit amount
  balance_after,             -- INT: Balance after transaction
  attach_idx,                -- BIGINT: Generated image ID
  production_idx,            -- UUID: Production ID
  description,               -- TEXT: Transaction description
  created_at                 -- TIMESTAMP: NOW()
) VALUES (...)
```

---

## Image Type Categories

### Attachment Type Field (from individualImageAttachIds)
```
type: "model"        -- Model/person image
type: "top"          -- Clothing top
type: "pants"        -- Pants/bottom
type: "outer"        -- Outer clothing/jacket
type: "shoes"        -- Shoes
type: "bag"          -- Bag
type: "accessory"    -- General accessory
type: "acce"         -- Short form of accessory
type: "background"   -- Background/environment
type: "bg"           -- Short form of background
```

---

## Complete Query Flow Example

### For a single Stage 0 image (attachId: 39070):

```
1. FETCH JOB CONFIG:
   SELECT * FROM quel_production_jobs WHERE job_id = 'job-uuid'
   -> Returns job_input_data with stages[0].individualImageAttachIds = [39070]

2. FETCH IMAGE METADATA:
   SELECT * FROM quel_attach WHERE attach_id = 39070
   -> Returns attach_file_path = "uploads/original/model.webp"

3. DOWNLOAD IMAGE:
   GET ${SupabaseStorageURL}/uploads/original/model.webp
   -> Returns image bytes

4. GENERATE NEW IMAGE:
   Gemini API (model image + stage prompt)
   -> Returns base64 PNG

5. UPLOAD NEW IMAGE:
   POST ${SupabaseStorageURL}/uploads/generated/fashion/new.webp
   -> Returns filePath

6. CREATE ATTACH RECORD:
   INSERT INTO quel_attach (
     attach_file_path = "uploads/generated/fashion/new.webp",
     attach_file_size = 12345,
     attach_file_type = "image/webp",
     ...
   )
   -> RETURNING attach_id = 39100

7. DEDUCT CREDITS:
   SELECT org_credit FROM quel_organization WHERE org_id = 'org-id'
   -> Returns 1000
   
   UPDATE quel_organization SET org_credit = 980 WHERE org_id = 'org-id'
   
   INSERT INTO quel_credits (
     transaction_type = 'deduction',
     amount = 20,
     balance_after = 980,
     attach_idx = 39100,
     ...
   )

8. UPDATE JOB PROGRESS:
   UPDATE quel_production_jobs
   SET completed_images = 1,
       generated_attach_ids = '[39100]'
   WHERE job_id = 'job-uuid'

9. FINAL UPDATES:
   SELECT attach_ids FROM quel_production_photo WHERE production_id = 'prod-id'
   -> Returns existing = [39070] (input IDs stored)
   
   UPDATE quel_production_photo
   SET attach_ids = '[39070, 39100]'
   WHERE production_id = 'prod-id'
```

---

## Critical Columns Summary

| Table | Column | Type | Purpose | Example |
|-------|--------|------|---------|---------|
| quel_production_jobs | job_input_data | JSONB | Configuration & image IDs | See above structures |
| quel_production_jobs | generated_attach_ids | JSONB | Output image IDs | [39100, 39101] |
| quel_production_jobs | job_status | ENUM | Processing state | pending→processing→completed |
| quel_attach | attach_file_path | VARCHAR | Download location | uploads/generated/... |
| quel_attach | attach_id | BIGINT | Image identifier | 39070, 39100 |
| quel_production_photo | attach_ids | JSONB | Final image list | [39100, 39101] |
| quel_organization | org_credit | INT | Credit balance | 1000 |

