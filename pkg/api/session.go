package api

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"zotregistry.io/zot/pkg/extensions/monitoring"
	"zotregistry.io/zot/pkg/log"
)

type statusWriter struct {
	http.ResponseWriter
	status int
	length int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}

	n, err := w.ResponseWriter.Write(b)
	w.length += n

	return n, err
}

// SessionLogger logs session details.
func SessionLogger(c *Controller) mux.MiddlewareFunc {
	logger := c.Log.With().Str("module", "http").Logger()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			// Start timer
			start := time.Now()
			path := request.URL.Path
			raw := request.URL.RawQuery

			sw := statusWriter{ResponseWriter: w}

			// Process request
			next.ServeHTTP(&sw, request)

			// Stop timer
			end := time.Now()
			latency := end.Sub(start)
			if latency > time.Minute {
				// Truncate in a golang < 1.8 safe way
				latency -= latency % time.Second
			}
			clientIP := request.RemoteAddr
			method := request.Method
			headers := map[string][]string{}
			log := logger.Info()
			for key, value := range request.Header {
				if key == "Authorization" { // anonymize from logs
					s := strings.SplitN(value[0], " ", 2)
					if len(s) == 2 && strings.EqualFold(s[0], "basic") {
						b, err := base64.StdEncoding.DecodeString(s[1])
						if err == nil {
							pair := strings.SplitN(string(b), ":", 2)
							// nolint:gomnd
							if len(pair) == 2 {
								log = log.Str("username", pair[0])
							}
						}
					}
					value = []string{"******"}
				}
				headers[key] = value
			}
			statusCode := sw.status
			bodySize := sw.length
			if raw != "" {
				path = path + "?" + raw
			}

			if path != "/v2/metrics" {
				// In order to test metrics feture,the instrumentation related to node exporter
				// should be handled by node exporter itself (ex: latency)
				monitoring.IncHTTPConnRequests(c.Metrics, method, strconv.Itoa(statusCode))
				monitoring.ObserveHTTPRepoLatency(c.Metrics, path, latency)     // summary
				monitoring.ObserveHTTPMethodLatency(c.Metrics, method, latency) // histogram
			}

			log.Str("clientIP", clientIP).
				Str("method", method).
				Str("path", path).
				Int("statusCode", statusCode).
				Str("latency", latency.String()).
				Int("bodySize", bodySize).
				Interface("headers", headers).
				Msg("HTTP API")
		})
	}
}

func SessionAuditLogger(audit *log.Logger) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			raw := r.URL.RawQuery

			sw := statusWriter{ResponseWriter: w}

			// Process request
			next.ServeHTTP(&sw, r)

			clientIP := r.RemoteAddr
			method := r.Method
			username := ""

			for key, value := range r.Header {
				if key == "Authorization" { // anonymize from logs
					s := strings.SplitN(value[0], " ", 2)
					if len(s) == 2 && strings.EqualFold(s[0], "basic") {
						b, err := base64.StdEncoding.DecodeString(s[1])
						if err == nil {
							pair := strings.SplitN(string(b), ":", 2)
							// nolint:gomnd
							if len(pair) == 2 {
								username = pair[0]
							}
						}
					}
				}
			}

			statusCode := sw.status
			if raw != "" {
				path = path + "?" + raw
			}

			if (method == http.MethodPost || method == http.MethodPut ||
				method == http.MethodPatch || method == http.MethodDelete) &&
				(statusCode == http.StatusOK || statusCode == http.StatusCreated || statusCode == http.StatusAccepted) {
				audit.Info().
					Str("clientIP", clientIP).
					Str("subject", username).
					Str("action", method).
					Str("object", path).
					Int("status", statusCode).
					Msg("HTTP API Audit")
			}
		})
	}
}
