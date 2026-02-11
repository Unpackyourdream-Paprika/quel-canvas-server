import re
import sys
import io

# Force UTF-8 output
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')

def update_file(filepath):
    print(f"Processing {filepath}...")
    
    with open(filepath, 'r', encoding='utf-8') as f:
        content = f.read()
    
    original_content = content
    
    # 1. Replace genai.NewClient initialization
    pattern1 = r'genaiClient, err := genai\.NewClient\(ctx, &genai\.ClientConfig\{\s*APIKey:\s*cfg\.GeminiAPIKey,\s*Backend:\s*genai\.BackendGeminiAPI,\s*\}\)'
    replacement1 = 'genaiClient, err := vertexai.NewVertexAIClient(ctx, cfg.VertexAIProject, cfg.VertexAILocation)'
    content = re.sub(pattern1, replacement1, content, flags=re.MULTILINE | re.DOTALL)
    
    # 2. Replace genai.NewPartFromText with genai.Text
    content = content.replace('genai.NewPartFromText(', 'genai.Text(')
    
    # 3. Replace &genai.Part{InlineData: &genai.Blob{...}} with genai.ImageData(...)
    pattern3 = r'&genai\.Part\{\s*InlineData:\s*&genai\.Blob\{\s*MIMEType:\s*"([^"]+)",\s*Data:\s*([^,\}]+),?\s*\}\s*\}'
    replacement3 = r'genai.ImageData("\1", \2)'
    content = re.sub(pattern3, replacement3, content, flags=re.MULTILINE | re.DOTALL)
    
    # 4. Replace InlineData checks with Blob type assertions
    # Pattern: if part.InlineData != nil && len(part.InlineData.Data) > 0 {
    pattern4a = r'if part\.InlineData != nil && len\(part\.InlineData\.Data\) > 0 \{'
    replacement4a = 'if blob, ok := part.(genai.Blob); ok {\n\t\t\tif len(blob.Data) > 0 {'
    content = re.sub(pattern4a, replacement4a, content)
    
    # Pattern: data := part.InlineData.Data
    content = content.replace('part.InlineData.Data', 'blob.Data')
    
    if content != original_content:
        with open(filepath, 'w', encoding='utf-8') as f:
            f.write(content)
        print(f"[OK] Updated {filepath}")
        return True
    else:
        print(f"[SKIP] No changes needed for {filepath}")
        return False

files = [
    "modules/beauty/service.go",
    "modules/cartoon/service.go",
    "modules/cinema/service.go",
    "modules/eats/service.go",
    "modules/fashion/service.go",
    "modules/generate-image/service.go",
    "modules/modify/worker.go",
    "modules/multiview/service.go",
    "modules/multiview/worker.go",
    "modules/submodule/nanobanana/service.go",
    "modules/unified-prompt/landing/service.go",
    "modules/unified-prompt/studio/service.go",
]

updated_count = 0
for filepath in files:
    try:
        if update_file(filepath):
            updated_count += 1
    except Exception as e:
        print(f"[ERROR] Error processing {filepath}: {e}")

print(f"\n=== Summary: Updated {updated_count}/{len(files)} files ===")
