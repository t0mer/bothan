package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/t0mer/bothan/internal/config"
	"github.com/t0mer/bothan/internal/settings"
)

// SettingsService is the settings behaviour the handlers depend on.
type SettingsService interface {
	Current() *settings.Settings
	Raw(key string) string
	Update(ctx context.Context, patch map[string]string) error
	Bootstrap() config.Bootstrap
	EnvOverriddenBind() []string
}

// Settings holds the settings resource handlers.
type Settings struct {
	svc SettingsService
}

// NewSettings builds the settings handlers.
func NewSettings(svc SettingsService) *Settings { return &Settings{svc: svc} }

// Routes mounts the settings endpoints onto r.
func (h *Settings) Routes(r chi.Router) {
	r.Get("/", h.get)
	r.Put("/", h.update)
}

// --- response DTOs (durations exposed as strings; secrets never emitted) ---

type settingsResponse struct {
	Server    serverDTO    `json:"server"`
	Log       logDTO       `json:"log"`
	SSLLabs   ssllabsDTO   `json:"ssllabs"`
	Metrics   metricsDTO   `json:"metrics"`
	Auth      authDTO      `json:"auth"`
	Bootstrap bootstrapDTO `json:"bootstrap"`
}

type authDTO struct {
	Enabled         bool `json:"enabled"`
	ProtectMetrics  bool `json:"protect_metrics"`
	RestartRequired bool `json:"restart_required"`
}

type serverDTO struct {
	Host            string   `json:"host"`
	Port            int      `json:"port"`
	BasePath        string   `json:"base_path"`
	EnvOverridden   []string `json:"env_overridden"`
	RestartRequired bool     `json:"restart_required"`
}

type logDTO struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

type ssllabsDTO struct {
	APIVersion     string `json:"api_version"`
	Email          string `json:"email"`
	PollInterval   string `json:"poll_interval"`
	MaxWorkers     int    `json:"max_workers"`
	ScanTimeout    string `json:"scan_timeout"`
	DefaultPublish bool   `json:"default_publish"`
}

type metricsDTO struct {
	Enabled         bool `json:"enabled"`
	RestartRequired bool `json:"restart_required"`
}

type bootstrapDTO struct {
	DatabasePath     string `json:"database_path"`
	EncryptionKeySet bool   `json:"encryption_key_set"`
}

func (h *Settings) get(w http.ResponseWriter, _ *http.Request) {
	cur := h.svc.Current()
	bs := h.svc.Bootstrap()
	resp := settingsResponse{
		Server: serverDTO{
			Host:            cur.Server.Host,
			Port:            cur.Server.Port,
			BasePath:        cur.Server.BasePath,
			EnvOverridden:   h.svc.EnvOverriddenBind(),
			RestartRequired: true,
		},
		Log: logDTO{Level: cur.Log.Level, Format: cur.Log.Format},
		SSLLabs: ssllabsDTO{
			APIVersion:     cur.SSLLabs.APIVersion,
			Email:          cur.SSLLabs.Email,
			PollInterval:   h.svc.Raw(settings.KeySSLLabsPollInterval),
			MaxWorkers:     cur.SSLLabs.MaxWorkers,
			ScanTimeout:    h.svc.Raw(settings.KeySSLLabsScanTimeout),
			DefaultPublish: cur.SSLLabs.DefaultPublish,
		},
		Metrics:   metricsDTO{Enabled: cur.Metrics.Enabled, RestartRequired: true},
		Auth:      authDTO{Enabled: cur.Auth.Enabled, ProtectMetrics: cur.Auth.ProtectMetrics, RestartRequired: true},
		Bootstrap: bootstrapDTO{DatabasePath: bs.DatabasePath, EncryptionKeySet: bs.EncryptionKey != ""},
	}
	WriteJSON(w, http.StatusOK, resp)
}

// settingsPatch is the partial update payload; only present fields are applied.
type settingsPatch struct {
	Server *struct {
		Host     *string `json:"host"`
		Port     *int    `json:"port"`
		BasePath *string `json:"base_path"`
	} `json:"server"`
	Log *struct {
		Level  *string `json:"level"`
		Format *string `json:"format"`
	} `json:"log"`
	SSLLabs *struct {
		APIVersion     *string `json:"api_version"`
		Email          *string `json:"email"`
		PollInterval   *string `json:"poll_interval"`
		MaxWorkers     *int    `json:"max_workers"`
		ScanTimeout    *string `json:"scan_timeout"`
		DefaultPublish *bool   `json:"default_publish"`
	} `json:"ssllabs"`
	Metrics *struct {
		Enabled *bool `json:"enabled"`
	} `json:"metrics"`
	Auth *struct {
		Enabled        *bool `json:"enabled"`
		ProtectMetrics *bool `json:"protect_metrics"`
	} `json:"auth"`
}

func (h *Settings) update(w http.ResponseWriter, r *http.Request) {
	var p settingsPatch
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid", "malformed JSON body: "+err.Error())
		return
	}

	patch := map[string]string{}
	if p.Server != nil {
		putString(patch, settings.KeyServerHost, p.Server.Host)
		putInt(patch, settings.KeyServerPort, p.Server.Port)
		putString(patch, settings.KeyServerBasePath, p.Server.BasePath)
	}
	if p.Log != nil {
		putString(patch, settings.KeyLogLevel, p.Log.Level)
		putString(patch, settings.KeyLogFormat, p.Log.Format)
	}
	if p.SSLLabs != nil {
		putString(patch, settings.KeySSLLabsAPIVersion, p.SSLLabs.APIVersion)
		putString(patch, settings.KeySSLLabsEmail, p.SSLLabs.Email)
		putString(patch, settings.KeySSLLabsPollInterval, p.SSLLabs.PollInterval)
		putInt(patch, settings.KeySSLLabsMaxWorkers, p.SSLLabs.MaxWorkers)
		putString(patch, settings.KeySSLLabsScanTimeout, p.SSLLabs.ScanTimeout)
		putBool(patch, settings.KeySSLLabsDefaultPublish, p.SSLLabs.DefaultPublish)
	}
	if p.Metrics != nil {
		putBool(patch, settings.KeyMetricsEnabled, p.Metrics.Enabled)
	}
	if p.Auth != nil {
		putBool(patch, settings.KeyAuthEnabled, p.Auth.Enabled)
		putBool(patch, settings.KeyAuthProtectMetrics, p.Auth.ProtectMetrics)
	}

	if len(patch) == 0 {
		WriteError(w, http.StatusBadRequest, "invalid", "no settings provided")
		return
	}
	if err := h.svc.Update(r.Context(), patch); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid", err.Error())
		return
	}
	h.get(w, r)
}

func putString(m map[string]string, key string, v *string) {
	if v != nil {
		m[key] = *v
	}
}
func putInt(m map[string]string, key string, v *int) {
	if v != nil {
		m[key] = strconv.Itoa(*v)
	}
}
func putBool(m map[string]string, key string, v *bool) {
	if v != nil {
		m[key] = strconv.FormatBool(*v)
	}
}
