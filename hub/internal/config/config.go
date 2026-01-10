package config

// Config holds the hub configuration
type Config struct {
	Port        string
	OpenCodeURL string
	Token       string

	// Phase 2: Agent and Redis
	AgentToken string // Token for agent authentication
	RedisAddr  string // Redis address (empty = disabled)
	RedisPass  string // Redis password
	RedisDB    int    // Redis database number
}

// New creates a default configuration
func New() *Config {
	return &Config{
		Port:        "8080",
		OpenCodeURL: "http://localhost:4096",
		Token:       "",
		AgentToken:  "",
		RedisAddr:   "",
		RedisPass:   "",
		RedisDB:     0,
	}
}
