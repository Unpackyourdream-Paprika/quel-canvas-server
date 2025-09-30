package generateimage

import (
	"log"
)

type Service struct {
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) GenerateImage() string {
	log.Println("🎨 GenerateImage 함수 도달!")
	return "POST"
}

func (s *Service) GetImageStatus() string {
	log.Println("📊 GetImageStatus 함수 도달!")
	return "GET"
}