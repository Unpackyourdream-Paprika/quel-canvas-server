package utils

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // JPEG 디코더 등록
	"image/png"
	"log"
	"math"

	"github.com/gen2brain/webp"
)

// ConvertImageToBase64 - 이미지 바이너리를 base64로 변환
func ConvertImageToBase64(imageData []byte) string {
	base64Str := base64.StdEncoding.EncodeToString(imageData)
	log.Printf("🔄 Image converted to base64: %d chars (preview: %s...)",
		len(base64Str),
		base64Str[:min(50, len(base64Str))])
	return base64Str
}

// ConvertPNGToWebP - PNG 바이너리를 WebP로 변환
func ConvertPNGToWebP(pngData []byte, quality float32) ([]byte, error) {
	log.Printf("🔄 Converting PNG to WebP (quality: %.1f)", quality)

	// PNG 디코딩
	pngReader := bytes.NewReader(pngData)
	img, err := png.Decode(pngReader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode PNG: %w", err)
	}

	// WebP 인코딩 (gen2brain/webp 사용)
	var webpBuffer bytes.Buffer
	err = webp.Encode(&webpBuffer, img, webp.Options{Quality: int(quality)})
	if err != nil {
		return nil, fmt.Errorf("failed to encode WebP: %w", err)
	}

	webpData := webpBuffer.Bytes()

	log.Printf("✅ PNG converted to WebP: %d bytes → %d bytes (%.1f%% reduction)",
		len(pngData), len(webpData),
		float64(len(pngData)-len(webpData))/float64(len(pngData))*100)

	return webpData, nil
}

// MergeImages - 여러 이미지를 Grid 방식으로 병합
func MergeImages(images [][]byte, aspectRatio string) ([]byte, error) {
	if len(images) == 0 {
		return nil, fmt.Errorf("no images to merge")
	}

	if len(images) == 1 {
		// 단일 이미지는 원본 그대로 반환
		log.Printf("✅ Single image - returning original")
		return images[0], nil
	}

	// 이미지 디코드
	decodedImages := []image.Image{}
	for i, imgData := range images {
		img, format, err := image.Decode(bytes.NewReader(imgData))
		if err != nil {
			log.Printf("⚠️  Failed to decode image %d: %v", i, err)
			continue
		}
		log.Printf("🔍 Decoded image %d format: %s, size: %dx%d", i, format, img.Bounds().Dx(), img.Bounds().Dy())
		decodedImages = append(decodedImages, img)
	}

	if len(decodedImages) == 0 {
		return nil, fmt.Errorf("no valid images to merge")
	}

	// Grid 방식으로 배치 (2x2, 2x3 등)
	numImages := len(decodedImages)
	cols := int(math.Ceil(math.Sqrt(float64(numImages))))      // 열 개수
	rows := int(math.Ceil(float64(numImages) / float64(cols))) // 행 개수

	// 각 셀의 최대 너비/높이 계산
	maxCellWidth := 0
	maxCellHeight := 0
	for _, img := range decodedImages {
		bounds := img.Bounds()
		if bounds.Dx() > maxCellWidth {
			maxCellWidth = bounds.Dx()
		}
		if bounds.Dy() > maxCellHeight {
			maxCellHeight = bounds.Dy()
		}
	}

	// 전체 그리드 크기
	totalWidth := cols * maxCellWidth
	totalHeight := rows * maxCellHeight

	// 새 이미지 생성
	merged := image.NewRGBA(image.Rect(0, 0, totalWidth, totalHeight))

	// Grid에 이미지 배치
	for idx, img := range decodedImages {
		row := idx / cols
		col := idx % cols

		x := col * maxCellWidth
		y := row * maxCellHeight

		bounds := img.Bounds()
		// 중앙 정렬
		xOffset := x + (maxCellWidth-bounds.Dx())/2
		yOffset := y + (maxCellHeight-bounds.Dy())/2

		draw.Draw(merged,
			image.Rect(xOffset, yOffset, xOffset+bounds.Dx(), yOffset+bounds.Dy()),
			img, image.Point{0, 0}, draw.Src)
	}

	log.Printf("✅ Merged %d images into %dx%d grid (%dx%d total)", len(decodedImages), rows, cols, totalWidth, totalHeight)

	// aspect-ratio에 따른 목표 크기 설정
	var targetWidth, targetHeight int
	switch aspectRatio {
	case "16:9":
		targetWidth, targetHeight = 2048, 1152
	case "9:16":
		targetWidth, targetHeight = 1152, 2048
	default:
		// 1:1, 4:3, 3:4 등 나머지는 모두 2048x2048
		targetWidth, targetHeight = 2048, 2048
	}

	// 병합된 이미지를 목표 크기로 리사이즈
	finalImage := ResizeImage(merged, targetWidth, targetHeight)
	log.Printf("✅ Resized merged grid to %dx%d (aspect-ratio: %s)", targetWidth, targetHeight, aspectRatio)

	// PNG 인코딩
	var buf bytes.Buffer
	if err := png.Encode(&buf, finalImage); err != nil {
		return nil, fmt.Errorf("failed to encode merged image: %w", err)
	}

	return buf.Bytes(), nil
}

// ResizeImage - 이미지를 지정된 크기로 resize (비율 유지하며 fit, 투명 배경)
func ResizeImage(src image.Image, targetWidth, targetHeight int) image.Image {
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()

	// 비율 계산
	scaleX := float64(targetWidth) / float64(srcWidth)
	scaleY := float64(targetHeight) / float64(srcHeight)
	scale := math.Min(scaleX, scaleY)

	// 스케일된 크기 계산
	newWidth := int(float64(srcWidth) * scale)
	newHeight := int(float64(srcHeight) * scale)

	// 새 이미지 생성 (목표 크기, 검은 배경)
	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))

	// 중앙 정렬을 위한 오프셋 계산
	xOffset := (targetWidth - newWidth) / 2
	yOffset := (targetHeight - newHeight) / 2

	// Nearest Neighbor 방식으로 리사이즈
	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := int(float64(x) / scale)
			srcY := int(float64(y) / scale)
			dst.Set(x+xOffset, y+yOffset, src.At(srcX, srcY))
		}
	}

	return dst
}

// min helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
