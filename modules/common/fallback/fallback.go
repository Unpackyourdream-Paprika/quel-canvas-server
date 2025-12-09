package fallback

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"strconv"
	"strings"
)

const transparentPixelBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMB/6X+ZQAAAABJRU5ErkJggg=="

var transparentPixelBytes []byte

func init() {
	data, err := base64.StdEncoding.DecodeString(transparentPixelBase64)
	if err != nil {
		log.Printf("⚠️ Failed to decode placeholder pixel: %v", err)
		return
	}
	transparentPixelBytes = data
}

// PlaceholderBase64 returns a 1x1 transparent PNG in base64 for slots that have no source image.
func PlaceholderBase64() string {
	return transparentPixelBase64
}

// PlaceholderBytes returns a copy of the transparent PNG bytes.
func PlaceholderBytes() []byte {
	if len(transparentPixelBytes) == 0 {
		return []byte{}
	}
	out := make([]byte, len(transparentPixelBytes))
	copy(out, transparentPixelBytes)
	return out
}

// SafeString returns a trimmed string or the provided fallback.
func SafeString(value interface{}, fallback string) string {
	if s, ok := value.(string); ok {
		s = strings.TrimSpace(s)
		if s != "" {
			return s
		}
	}
	return fallback
}

// SafeInt converts common number shapes into int with a fallback.
func SafeInt(value interface{}, fallback int) int {
	switch v := value.(type) {
	case float64:
		if v > 0 {
			return int(v)
		}
	case float32:
		if v > 0 {
			return int(v)
		}
	case int:
		if v > 0 {
			return v
		}
	case int64:
		if v > 0 {
			return int(v)
		}
	case json.Number:
		if n, err := strconv.Atoi(v.String()); err == nil && n > 0 {
			return n
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

// SafeAspectRatio provides a sane default aspect ratio.
func SafeAspectRatio(value interface{}) string {
	return SafeString(value, "16:9")
}

// DefaultQuantity uses job total images or falls back to 1.
func DefaultQuantity(totalImages int) int {
	if totalImages > 0 {
		return totalImages
	}
	return 1
}

// NormalizeCombinations produces at least one combination with safe defaults.
func NormalizeCombinations(raw interface{}, defaultQuantity int, defaultAngle, defaultShot string) []map[string]interface{} {
	combos := []map[string]interface{}{}

	if list, ok := raw.([]interface{}); ok {
		for _, item := range list {
			if m, ok := item.(map[string]interface{}); ok {
				combos = append(combos, normalizeComboMap(m, defaultQuantity, defaultAngle, defaultShot))
			}
		}
	}

	if len(combos) == 0 {
		combos = append(combos, normalizeComboMap(map[string]interface{}{}, defaultQuantity, defaultAngle, defaultShot))
	}

	return combos
}

func normalizeComboMap(m map[string]interface{}, defaultQuantity int, defaultAngle, defaultShot string) map[string]interface{} {
	quantity := SafeInt(m["quantity"], defaultQuantity)
	if quantity <= 0 {
		quantity = defaultQuantity
	}

	return map[string]interface{}{
		"angle":    SafeString(m["angle"], defaultAngle),
		"shot":     SafeString(m["shot"], defaultShot),
		"quantity": quantity,
	}
}
