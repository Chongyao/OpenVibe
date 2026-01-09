package config

// Config holds the hub configuration
type Config struct {
	Port        string
	OpenCodeURL string
	Token       string
}

// New creates a default configuration
func New() *Config {
	return &Config{
		Port:        "8080",
		OpenCodeURL: "http://localhost:4096",
		Token:       "",
	}
}
