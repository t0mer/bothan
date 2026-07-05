// Package config loads and validates Bothan's configuration.
//
// Precedence follows the house convention: flags > env > YAML > built-in
// defaults. Environment variables use the BOTHAN_ prefix with dots mapped to
// underscores (e.g. server.port -> BOTHAN_SERVER_PORT).
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config is the fully resolved application configuration.
type Config struct {
	Server   Server   `mapstructure:"server"`
	Database Database `mapstructure:"database"`
	SSLLabs  SSLLabs  `mapstructure:"ssllabs"`
	Crypto   Crypto   `mapstructure:"crypto"`
	Auth     Auth     `mapstructure:"auth"`
	Log      Log      `mapstructure:"log"`
	Metrics  Metrics  `mapstructure:"metrics"`
}

// Server holds HTTP server settings.
type Server struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	BasePath string `mapstructure:"base_path"`
}

// Database holds persistence settings.
type Database struct {
	Path string `mapstructure:"path"`
}

// SSLLabs holds Qualys SSL Labs client settings.
type SSLLabs struct {
	APIVersion     string        `mapstructure:"api_version"`
	Email          string        `mapstructure:"email"`
	PollInterval   time.Duration `mapstructure:"poll_interval"`
	MaxWorkers     int           `mapstructure:"max_workers"`
	ScanTimeout    time.Duration `mapstructure:"scan_timeout"`
	DefaultPublish bool          `mapstructure:"default_publish"`
}

// Crypto holds secret-at-rest settings.
type Crypto struct {
	// EncryptionKey is the 32-byte AES-256-GCM key (base64 or hex). It is
	// required once any channel exists; it is never logged.
	EncryptionKey string `mapstructure:"encryption_key"`
}

// Auth holds optional authentication settings.
type Auth struct {
	Enabled              bool   `mapstructure:"enabled"`
	SessionSecret        string `mapstructure:"session_secret"`
	InitialAdminUser     string `mapstructure:"initial_admin_user"`
	InitialAdminPassword string `mapstructure:"initial_admin_password"`
	ProtectMetrics       bool   `mapstructure:"protect_metrics"`
}

// Log holds logging settings.
type Log struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// Metrics holds Prometheus settings.
type Metrics struct {
	Enabled bool `mapstructure:"enabled"`
}

// RegisterFlags registers Bothan's CLI flags on the given flag set. main() and
// tests share this so flag names stay consistent.
func RegisterFlags(fs *pflag.FlagSet) {
	fs.String("config", "", "path to config YAML file")
	fs.String("host", "", "HTTP listen host (server.host)")
	fs.Int("port", 0, "HTTP listen port (server.port)")
	fs.String("base-path", "", "base path for reverse-proxy sub-paths (server.base_path)")
	fs.String("db-path", "", "SQLite database path (database.path)")
	fs.String("log-level", "", "log level: debug|info|warn|error (log.level)")
	fs.String("log-format", "", "log format: json|text (log.format)")
	fs.Bool("version", false, "print version and exit")
}

// flagToKey maps a CLI flag name to its config key for viper binding.
var flagToKey = map[string]string{
	"host":       "server.host",
	"port":       "server.port",
	"base-path":  "server.base_path",
	"db-path":    "database.path",
	"log-level":  "log.level",
	"log-format": "log.format",
}

// Load resolves configuration from the given (already-parsed) flag set,
// environment, and an optional YAML file (the --config flag), then validates it.
func Load(fs *pflag.FlagSet) (*Config, error) {
	v := viper.New()
	setDefaults(v)

	v.SetEnvPrefix("BOTHAN")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	for flag, key := range flagToKey {
		if err := v.BindPFlag(key, fs.Lookup(flag)); err != nil {
			return nil, fmt.Errorf("binding flag %q: %w", flag, err)
		}
	}

	if path, _ := fs.GetString("config"); path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading config file %q: %w", path, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("decoding config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.base_path", "/")

	v.SetDefault("database.path", "/data/bothan.db")

	v.SetDefault("ssllabs.api_version", "v4")
	v.SetDefault("ssllabs.email", "")
	v.SetDefault("ssllabs.poll_interval", "10s")
	v.SetDefault("ssllabs.max_workers", 5)
	v.SetDefault("ssllabs.scan_timeout", "20m")
	v.SetDefault("ssllabs.default_publish", false)

	v.SetDefault("crypto.encryption_key", "")

	v.SetDefault("auth.enabled", false)
	v.SetDefault("auth.session_secret", "")
	v.SetDefault("auth.initial_admin_user", "")
	v.SetDefault("auth.initial_admin_password", "")
	v.SetDefault("auth.protect_metrics", false)

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	v.SetDefault("metrics.enabled", true)
}

// Validate reports whether the configuration is usable.
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port %d out of range (1-65535)", c.Server.Port)
	}
	switch c.SSLLabs.APIVersion {
	case "v4", "v3":
	default:
		return fmt.Errorf("ssllabs.api_version %q must be v4 or v3", c.SSLLabs.APIVersion)
	}
	if c.SSLLabs.PollInterval <= 0 {
		return fmt.Errorf("ssllabs.poll_interval must be positive")
	}
	if c.SSLLabs.ScanTimeout <= 0 {
		return fmt.Errorf("ssllabs.scan_timeout must be positive")
	}
	if c.SSLLabs.MaxWorkers < 1 {
		return fmt.Errorf("ssllabs.max_workers must be >= 1")
	}
	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("log.level %q must be debug|info|warn|error", c.Log.Level)
	}
	switch c.Log.Format {
	case "json", "text":
	default:
		return fmt.Errorf("log.format %q must be json|text", c.Log.Format)
	}
	return nil
}
