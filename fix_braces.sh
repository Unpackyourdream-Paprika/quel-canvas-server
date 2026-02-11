#!/bin/bash
# 모든 파일에서 누락된 중괄호 수정
for file in modules/beauty/service.go modules/cartoon/service.go modules/cinema/service.go modules/eats/service.go modules/fashion/service.go modules/unified-prompt/landing/service.go; do
  # 패턴: return ... nil 다음에 } 하나만 있고 } 하나 더 필요한 경우
  sed -i '/return base64.StdEncoding.EncodeToString(blob.Data), nil$/,/^[[:space:]]*}$/ {
    /^[[:space:]]*}$/ {
      a\		}
    }
  }' "$file"
  echo "Fixed: $file"
done
