// Package config resolves Bothan's bootstrap configuration — the minimal set of
// values that cannot live in the database because they are needed to open it or
// would be unsafe to store there.
//
// Everything else (server bind defaults, logging, SSL Labs, metrics, …) lives
// in the database and is edited from the Settings page; see internal/settings.
//
// Bootstrap precedence is flags > environment > built-in default.
package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/pflag"
)

const defaultDatabasePath = "/data/bothan.db"

// Bootstrap holds the env/flag-only configuration.
type Bootstrap struct {
	// DatabasePath is where the SQLite database lives.
	DatabasePath string
	// EncryptionKey is the AES-256-GCM key (base64/hex). Never stored in the DB.
	EncryptionKey string
	// SSLLabsBaseURL overrides the SSL Labs API base URL (self-hosted/testing);
	// empty uses the default for the configured API version.
	SSLLabsBaseURL string

	// ServerHost / ServerPort are optional overrides for the HTTP bind. When
	// set they win over the database value (so a container can pin its bind).
	// The *Set flags record whether an override was actually provided.
	ServerHost    string
	ServerHostSet bool
	ServerPort    int
	ServerPortSet bool
}

// RegisterFlags registers the bootstrap CLI flags. main() and tests share this.
func RegisterFlags(fs *pflag.FlagSet) {
	fs.String("db-path", "", "SQLite database path (env: BOTHAN_DATABASE_PATH)")
	fs.String("encryption-key", "", "AES-256-GCM key; prefer env BOTHAN_CRYPTO_ENCRYPTION_KEY")
	fs.String("host", "", "HTTP bind host override (env: BOTHAN_SERVER_HOST)")
	fs.Int("port", 0, "HTTP bind port override (env: BOTHAN_SERVER_PORT)")
	fs.Bool("version", false, "print version and exit")
}

// Load resolves the bootstrap config from the parsed flag set and environment.
func Load(fs *pflag.FlagSet) (*Bootstrap, error) {
	b := &Bootstrap{}

	b.DatabasePath = resolveString(fs, "db-path", "BOTHAN_DATABASE_PATH", defaultDatabasePath)
	b.EncryptionKey = resolveString(fs, "encryption-key", "BOTHAN_CRYPTO_ENCRYPTION_KEY", "")
	if v, ok := resolveOptionalString(fs, "", "BOTHAN_SSLLABS_BASE_URL"); ok {
		b.SSLLabsBaseURL = v
	}

	if host, ok := resolveOptionalString(fs, "host", "BOTHAN_SERVER_HOST"); ok {
		b.ServerHost, b.ServerHostSet = host, true
	}
	if raw, ok := resolveOptionalString(fs, "port", "BOTHAN_SERVER_PORT"); ok {
		port, err := strconv.Atoi(raw)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid server port override %q", raw)
		}
		b.ServerPort, b.ServerPortSet = port, true
	}

	return b, nil
}

// resolveString applies flag > env > default.
func resolveString(fs *pflag.FlagSet, flag, env, def string) string {
	if v, ok := resolveOptionalString(fs, flag, env); ok {
		return v
	}
	return def
}

// resolveOptionalString returns the flag value if the flag was set, else the
// env value if present, and reports whether either was provided.
func resolveOptionalString(fs *pflag.FlagSet, flag, env string) (string, bool) {
	if fs.Changed(flag) {
		return fs.Lookup(flag).Value.String(), true
	}
	if v, ok := os.LookupEnv(env); ok && v != "" {
		return v, true
	}
	return "", false
}
