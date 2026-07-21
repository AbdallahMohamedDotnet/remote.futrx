package config

import "os"

const CodeServerBaseURL = "https://code.remote.futrx.dev/"

type Config struct {
	Host    string
	Port    string
	DataDir string
	BaseURL string
}

func Load() Config {
	return Config{
		Host:    envDefault("HOST", "127.0.0.1"),
		Port:    envDefault("PORT", "7682"),
		DataDir: envDefault("DATA_DIR", "/opt/remote.futrx.dev/data"),
		BaseURL: envDefault("BASE_URL", ""),
	}
}

func (c Config) Addr() string {
	return c.Host + ":" + c.Port
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
