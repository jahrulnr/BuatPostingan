package config

import (
	"os"
	"strconv"
	"strings"
)

// Config is env-based runtime config (no external cleanenv yet).
type Config struct {
	HTTPAddr      string
	WebRoot       string
	StorageRoot   string
	DocsRoot      string
	WriteEnabled  bool
	MaxToolRounds int
	SpeakFloorTTL int
}

func Load() Config {
	return Config{
		HTTPAddr:      getenv("BP_HTTP_ADDR", ":8080"),
		WebRoot:       getenv("BP_WEB_ROOT", "web"),
		StorageRoot:   getenv("BP_STORAGE_ROOT", "storage/webchat"),
		DocsRoot:      getenv("BP_DOCS_ROOT", "docs/webchat"),
		WriteEnabled:  false, // hard false — reader/instructor only
		MaxToolRounds: getenvInt("BP_MAX_TOOL_ROUNDS", 8),
		SpeakFloorTTL: getenvInt("BP_SPEAK_FLOOR_TTL_SEC", 600),
	}
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
