#!/bin/bash

# List of files to update
files=(
    "modules/beauty/service.go"
    "modules/cartoon/service.go"
    "modules/cinema/service.go"
    "modules/eats/service.go"
    "modules/fashion/service.go"
    "modules/generate-image/service.go"
    "modules/modify/worker.go"
    "modules/multiview/service.go"
    "modules/multiview/worker.go"
    "modules/submodule/nanobanana/service.go"
    "modules/unified-prompt/landing/service.go"
    "modules/unified-prompt/studio/service.go"
)

for file in "${files[@]}"; do
    echo "Processing $file..."
    
    # Create backup
    cp "$file" "$file.bak"
    
    # 1. Replace genai.NewClient with vertexai.NewVertexAIClient
    sed -i 's/genaiClient, err := genai\.NewClient(ctx, \&genai\.ClientConfig{/genaiClient, err := vertexai.NewVertexAIClient(ctx, cfg.VertexAIProject, cfg.VertexAILocation)/g' "$file"
    
    # 2. Remove the APIKey and Backend lines (they appear after NewClient)
    sed -i '/APIKey:  *cfg\.GeminiAPIKey,/d' "$file"
    sed -i '/Backend: *genai\.BackendGeminiAPI,/d' "$file"
    sed -i '/})/d' "$file"
    
    # 3. Replace InlineData checks with Blob type assertions
    sed -i 's/if part\.InlineData != nil && len(part\.InlineData\.Data) > 0 {/if blob, ok := part.(genai.Blob); ok {\n\t\t\tif len(blob.Data) > 0 {/g' "$file"
    sed -i 's/data := part\.InlineData\.Data/data := blob.Data/g' "$file"
    
    # 4. Replace Part creation patterns
    sed -i 's/genai\.NewPartFromText(/genai.Text(/g' "$file"
    sed -i 's/\&genai\.Part{InlineData: \&genai\.Blob{MIMEType: "\([^"]*\)", Data: \([^}]*\)}}/genai.ImageData("\1", \2)/g' "$file"
    
    echo "Completed $file"
done

echo "All files updated!"
