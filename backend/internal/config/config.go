package config

import (
	"errors"
	"net"
	"net/url"
	"os"
)

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
		DataDir: envDefault("DATA_DIR", "/opt/remote.futrx/data"),
		BaseURL: envDefault("BASE_URL", ""),
	}
}

func (c Config) Addr() string {
	return c.Host + ":" + c.Port
}

// CodeServerBaseURL derives the IDE origin from the public hostname selected
// during installation. For example, https://remote.example.com becomes
// https://code.remote.example.com/.
func CodeServerBaseURL(baseURL string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		return "", errors.New("BASE_URL must be an absolute URL")
	}
	host := "code." + parsed.Hostname()
	if port := parsed.Port(); port != "" {
		host = net.JoinHostPort(host, port)
	}
	parsed.Host = host
	parsed.Path = "/"
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
