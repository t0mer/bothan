// Package settings holds Bothan's database-backed, runtime-editable
// configuration. A typed, validated snapshot is built from key/value rows and
// served to the rest of the app; the Settings page edits it via the API.
//
// Bootstrap values (database path, encryption key) are NOT here — see
// internal/config.
package settings

import (
	"fmt"
	"strconv"
	"time"
)

// Setting keys, namespaced to mirror the typed sections below.
const (
	KeyServerHost     = "server.host"
	KeyServerPort     = "server.port"
	KeyServerBasePath = "server.base_path"

	KeyLogLevel  = "log.level"
	KeyLogFormat = "log.format"

	KeySSLLabsAPIVersion     = "ssllabs.api_version"
	KeySSLLabsEmail          = "ssllabs.email"
	KeySSLLabsPollInterval   = "ssllabs.poll_interval"
	KeySSLLabsMaxWorkers     = "ssllabs.max_workers"
	KeySSLLabsScanTimeout    = "ssllabs.scan_timeout"
	KeySSLLabsDefaultPublish = "ssllabs.default_publish"

	KeyMetricsEnabled = "metrics.enabled"

	KeyAuthEnabled        = "auth.enabled"
	KeyAuthProtectMetrics = "auth.protect_metrics"
	KeyAuthSessionSecret  = "auth.session_secret"
)

// Settings is the typed, validated snapshot of all editable configuration.
type Settings struct {
	Server  ServerSettings  `json:"server"`
	Log     LogSettings     `json:"log"`
	SSLLabs SSLLabsSettings `json:"ssllabs"`
	Metrics MetricsSettings `json:"metrics"`
	Auth    AuthSettings    `json:"auth"`
}

// AuthSettings holds optional authentication settings.
type AuthSettings struct {
	Enabled        bool   `json:"enabled"`
	ProtectMetrics bool   `json:"protect_metrics"`
	SessionSecret  string `json:"-"` // never serialized
}

// ServerSettings holds HTTP bind and routing settings.
type ServerSettings struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	BasePath string `json:"base_path"`
}

// LogSettings holds logging settings.
type LogSettings struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

// SSLLabsSettings holds Qualys SSL Labs client settings.
type SSLLabsSettings struct {
	APIVersion     string        `json:"api_version"`
	Email          string        `json:"email"`
	PollInterval   time.Duration `json:"-"`
	MaxWorkers     int           `json:"max_workers"`
	ScanTimeout    time.Duration `json:"-"`
	DefaultPublish bool          `json:"default_publish"`
}

// MetricsSettings holds Prometheus settings.
type MetricsSettings struct {
	Enabled bool `json:"enabled"`
}

// Defaults returns the seed key/value map for a fresh database.
func Defaults() map[string]string {
	return map[string]string{
		KeyServerHost:     "0.0.0.0",
		KeyServerPort:     "8080",
		KeyServerBasePath: "/",

		KeyLogLevel:  "info",
		KeyLogFormat: "json",

		KeySSLLabsAPIVersion:     "v4",
		KeySSLLabsEmail:          "",
		KeySSLLabsPollInterval:   "10s",
		KeySSLLabsMaxWorkers:     "5",
		KeySSLLabsScanTimeout:    "20m",
		KeySSLLabsDefaultPublish: "false",

		KeyMetricsEnabled: "true",

		KeyAuthEnabled:        "false",
		KeyAuthProtectMetrics: "false",
		KeyAuthSessionSecret:  "",
	}
}

// build converts a raw key/value map (merged over defaults) into a validated
// Settings snapshot.
func build(raw map[string]string) (*Settings, error) {
	get := func(key string) string {
		if v, ok := raw[key]; ok {
			return v
		}
		return Defaults()[key]
	}

	port, err := strconv.Atoi(get(KeyServerPort))
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("%s must be a port in 1-65535", KeyServerPort)
	}

	pollInterval, err := time.ParseDuration(get(KeySSLLabsPollInterval))
	if err != nil || pollInterval <= 0 {
		return nil, fmt.Errorf("%s must be a positive duration (e.g. 10s)", KeySSLLabsPollInterval)
	}
	scanTimeout, err := time.ParseDuration(get(KeySSLLabsScanTimeout))
	if err != nil || scanTimeout <= 0 {
		return nil, fmt.Errorf("%s must be a positive duration (e.g. 20m)", KeySSLLabsScanTimeout)
	}
	maxWorkers, err := strconv.Atoi(get(KeySSLLabsMaxWorkers))
	if err != nil || maxWorkers < 1 {
		return nil, fmt.Errorf("%s must be >= 1", KeySSLLabsMaxWorkers)
	}

	s := &Settings{
		Server: ServerSettings{
			Host:     get(KeyServerHost),
			Port:     port,
			BasePath: get(KeyServerBasePath),
		},
		Log: LogSettings{
			Level:  get(KeyLogLevel),
			Format: get(KeyLogFormat),
		},
		SSLLabs: SSLLabsSettings{
			APIVersion:     get(KeySSLLabsAPIVersion),
			Email:          get(KeySSLLabsEmail),
			PollInterval:   pollInterval,
			MaxWorkers:     maxWorkers,
			ScanTimeout:    scanTimeout,
			DefaultPublish: parseBool(get(KeySSLLabsDefaultPublish)),
		},
		Metrics: MetricsSettings{
			Enabled: parseBool(get(KeyMetricsEnabled)),
		},
		Auth: AuthSettings{
			Enabled:        parseBool(get(KeyAuthEnabled)),
			ProtectMetrics: parseBool(get(KeyAuthProtectMetrics)),
			SessionSecret:  get(KeyAuthSessionSecret),
		},
	}
	if err := validate(s); err != nil {
		return nil, err
	}
	return s, nil
}

func validate(s *Settings) error {
	switch s.SSLLabs.APIVersion {
	case "v4", "v3":
	default:
		return fmt.Errorf("%s must be v4 or v3", KeySSLLabsAPIVersion)
	}
	switch s.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("%s must be debug|info|warn|error", KeyLogLevel)
	}
	switch s.Log.Format {
	case "json", "text":
	default:
		return fmt.Errorf("%s must be json|text", KeyLogFormat)
	}
	if s.Server.BasePath == "" {
		return fmt.Errorf("%s must not be empty (use \"/\")", KeyServerBasePath)
	}
	return nil
}

func parseBool(s string) bool {
	b, _ := strconv.ParseBool(s)
	return b
}
