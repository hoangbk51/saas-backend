package services

import (
	"fmt"
	"go-saas/models"
)

func GetTryOnProvider(providerName string) (models.TryOnProvider, error) {
	switch providerName {
	case "tutaoweb":
		return &TutaowebProvider{}, nil
	case "fashn":
		return &FashnProvider{}, nil
	case "fitroom":
		return &FitroomProvider{}, nil
	case "google_virtual_tryon_preview":
		return &GoogleTryOnProvider{}, nil
	case "increai":
		return &IncreaiProvider{}, nil
	case "light_x":
		return &LightXProvider{}, nil
	case "miragic":
		return &MiragicProvider{}, nil
	case "modelslab":
		return &ModelsLabProvider{}, nil
	case "pixelcut":
		return &PixelcutProvider{}, nil
	case "virtual_tryon_gemini_flash":
		return &GooglePromptProvider{}, nil
	case "chatgpt_vton":
		return &ChatGPTermPromptProvider{}, nil
	default:
		return nil, fmt.Errorf("provider '%s' không hỗ trợ", providerName)
	}
}
