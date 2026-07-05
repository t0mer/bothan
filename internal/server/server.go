// Package server wires the chi HTTP router: middleware, system endpoints,
// the /api/v1 surface, the Prometheus handler, and the embedded SPA.
package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/t0mer/bothan/internal/api"
	"github.com/t0mer/bothan/internal/crypto"
	"github.com/t0mer/bothan/internal/metrics"
	"github.com/t0mer/bothan/internal/notify"
	"github.com/t0mer/bothan/internal/settings"
	"github.com/t0mer/bothan/internal/ssllabs"
	"github.com/t0mer/bothan/internal/store"
	"github.com/t0mer/bothan/internal/version"
	"github.com/t0mer/bothan/internal/web"
)

// Deps are the collaborators the HTTP server needs.
type Deps struct {
	Settings  *settings.Service
	Store     *store.Store
	Metrics   *metrics.Metrics
	Scanner   api.Scanner
	Scheduler api.SchedulerControl
	Cipher    *crypto.Cipher
	Logger    *slog.Logger
}

// newSSLLabsFactory builds a client factory for the info/registration
// endpoints, honouring the bootstrap base-URL override.
func newSSLLabsFactory(baseURL string) api.SSLLabsClientFactory {
	return func(s *settings.Settings) api.SSLLabsClient {
		return ssllabs.New(ssllabs.Options{
			APIVersion: s.SSLLabs.APIVersion,
			Email:      s.SSLLabs.Email,
			BaseURL:    baseURL,
		})
	}
}

// New builds the HTTP handler tree. The returned handler already accounts for
// server.base_path when serving behind a reverse-proxy sub-path.
//
// Bind (server.host/port), metrics enablement, and base_path are read once at
// construction; changing them takes effect on restart. SSL Labs and logging
// settings are read live where they are consumed.
func New(d Deps) (http.Handler, error) {
	cur := d.Settings.Current()

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger(d.Logger))
	r.Use(middleware.Recoverer)

	// System endpoints (unversioned, always reachable).
	r.Get("/healthz", healthHandler)
	r.Get("/readyz", readyHandler(d.Store))
	if cur.Metrics.Enabled {
		r.Handle("/metrics", d.Metrics.Handler())
	}

	// API v1.
	hosts := api.NewHosts(api.HostsDeps{
		Repo:           d.Store.Hosts(),
		DefaultPublish: func() bool { return d.Settings.Current().SSLLabs.DefaultPublish },
		Scanner:        d.Scanner,
		Scans:          d.Store.Scans(),
		Schedules:      d.Store.Schedules(),
		Channels:       d.Store.Channels(),
		Rules:          d.Store.Rules(),
		Scheduler:      d.Scheduler,
	})
	dispatcher := notify.NewDispatcher(nil)
	settingsHandler := api.NewSettings(d.Settings)
	scansHandler := api.NewScans(d.Store.Scans())
	schedulesHandler := api.NewSchedules(d.Store.Schedules(), d.Scheduler)
	channelsHandler := api.NewChannels(d.Store.Channels(), d.Cipher, dispatcher)
	rulesHandler := api.NewRules(d.Store.Rules())
	dashboardHandler := api.NewDashboard(d.Store.Dashboard())
	ssllabsHandler := api.NewSSLLabs(d.Settings, newSSLLabsFactory(d.Settings.Bootstrap().SSLLabsBaseURL))
	r.Route("/api/v1", func(v1 chi.Router) {
		v1.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			api.WriteError(w, http.StatusNotFound, "not_found", "no such API endpoint")
		})
		v1.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
			api.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		})
		v1.Route("/hosts", hosts.Routes)
		v1.Route("/settings", settingsHandler.Routes)
		v1.Route("/scans", scansHandler.Routes)
		v1.Route("/schedules", schedulesHandler.Routes)
		v1.Route("/channels", channelsHandler.Routes)
		v1.Route("/rules", rulesHandler.Routes)
		v1.Route("/dashboard", dashboardHandler.Routes)
		v1.Route("/ssllabs", ssllabsHandler.Routes)
	})

	// Embedded SPA as the catch-all (mounted last so API/system routes win).
	spa, err := web.Handler()
	if err != nil {
		return nil, err
	}
	r.NotFound(spa.ServeHTTP)
	r.MethodNotAllowed(spa.ServeHTTP)

	return applyBasePath(r, cur.Server.BasePath), nil
}

// applyBasePath strips a reverse-proxy sub-path prefix when configured.
func applyBasePath(h http.Handler, basePath string) http.Handler {
	bp := strings.Trim(basePath, "/")
	if bp == "" {
		return h
	}
	return http.StripPrefix("/"+bp, h)
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	api.WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": version.Version,
	})
}

func readyHandler(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := st.DB().PingContext(r.Context()); err != nil {
			api.WriteError(w, http.StatusServiceUnavailable, "not_ready", "database unavailable")
			return
		}
		api.WriteJSON(w, http.StatusOK, map[string]string{
			"status":  "ready",
			"version": version.Version,
		})
	}
}

// Addr formats a host:port listen address.
func Addr(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}
