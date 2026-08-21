package env

import (
	"os"
	"strings"
)

func HfApiBase() string {
	if v := os.Getenv("MODELS_HELPER_HF_API"); v != "" {
		return v
	}
	return "https://huggingface.co"
}

func ServerBaseUrl() string {
	if v := os.Getenv("MODELS_HELPER_SERVER_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://127.0.0.1:8081"
}

func TranscriptionServerBaseUrl() string {
	if v := os.Getenv("MODELS_HELPER_TRANSCRIPTION_SERVER_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return "http://127.0.0.1:8082"
}

func StudioBin() string {
	if v := os.Getenv("MODELS_HELPER_STUDIO_BIN"); v != "" {
		return v
	}
	return "studio"
}
