package service

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

type Server struct {
	config Config
	store  *Store
}

func New(config Config) (*Server, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	store, err := newStore(config)
	if err != nil {
		return nil, err
	}
	return &Server{config: config, store: store}, nil
}

func (s *Server) Serve(ctx context.Context) error {
	server := &http.Server{
		Addr:              s.config.Address,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    8 << 10,
	}
	errorsChannel := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errorsChannel <- err
		}
	}()
	select {
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownContext)
	case err := <-errorsChannel:
		return err
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.home)
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/v1/snapshots", s.authenticated(s.snapshots))
	mux.HandleFunc("/v1/summary", s.authenticated(s.summary))
	mux.HandleFunc("/v1/bundles", s.authenticated(s.bundles))
	mux.HandleFunc("/v1/bundles/", s.authenticated(s.bundle))
	return securityHeaders(mux)
}

func (s *Server) home(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" || request.Method != http.MethodGet {
		writeError(writer, http.StatusNotFound, "not found")
		return
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	if strings.Contains(strings.ToLower(request.Header.Get("Accept-Language")), "zh-tw") {
		_, _ = io.WriteString(writer, `<!doctype html><html lang="zh-Hant-TW"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>PastureStack 網路診斷</title></head><body><main><h1>PastureStack 網路診斷</h1><p>服務運作正常。診斷摘要與封包下載需要有效的管理權杖。</p><p><a href="/healthz">檢視服務健康狀態</a></p></main></body></html>`)
		return
	}
	_, _ = io.WriteString(writer, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>PastureStack Network Diagnostics</title></head><body><main><h1>PastureStack Network Diagnostics</h1><p>The service is running. Diagnostic summaries and bundle downloads require a valid administrative token.</p><p><a href="/healthz">View service health</a></p></main></body></html>`)
}

func (s *Server) health(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"status":  "healthy",
		"version": s.config.Version,
	})
}

func (s *Server) snapshots(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 64<<10)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var snapshot Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid snapshot")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeError(writer, http.StatusBadRequest, "invalid snapshot")
		return
	}
	if err := s.store.Add(snapshot); err != nil {
		if err.Error() == "agent limit reached" {
			writeError(writer, http.StatusConflict, "agent limit reached")
		} else {
			writeError(writer, http.StatusBadRequest, "invalid snapshot")
		}
		return
	}
	writeJSON(writer, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (s *Server) summary(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(writer, http.StatusOK, s.store.Summary())
}

func (s *Server) bundles(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		records, err := s.store.ListBundles()
		if err != nil {
			writeError(writer, http.StatusInternalServerError, "unable to list bundles")
			return
		}
		writeJSON(writer, http.StatusOK, bundleList{Data: records})
	case http.MethodPost:
		record, err := s.store.CreateBundle()
		if err != nil {
			if err.Error() == "no snapshots are available" {
				writeError(writer, http.StatusConflict, "no snapshots are available")
			} else {
				writeError(writer, http.StatusInternalServerError, "unable to create bundle")
			}
			return
		}
		writeJSON(writer, http.StatusCreated, record)
	default:
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) bundle(writer http.ResponseWriter, request *http.Request) {
	id := path.Base(request.URL.Path)
	if !bundleIDPattern.MatchString(id) {
		writeError(writer, http.StatusNotFound, "bundle not found")
		return
	}
	switch request.Method {
	case http.MethodGet:
		filePath, err := s.store.BundlePath(id)
		if err != nil {
			writeError(writer, http.StatusNotFound, "bundle not found")
			return
		}
		writer.Header().Set("Content-Type", "application/zip")
		writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="pasturestack-network-diagnostics-%s.zip"`, id))
		http.ServeFile(writer, request, filePath)
	case http.MethodDelete:
		if err := s.store.DeleteBundle(id); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				writeError(writer, http.StatusNotFound, "bundle not found")
			} else {
				writeError(writer, http.StatusInternalServerError, "unable to delete bundle")
			}
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	default:
		writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) authenticated(next http.HandlerFunc) http.HandlerFunc {
	expected := []byte("Bearer " + s.config.Token)
	return func(writer http.ResponseWriter, request *http.Request) {
		provided := []byte(request.Header.Get("Authorization"))
		if len(provided) != len(expected) || subtle.ConstantTimeCompare(provided, expected) != 1 {
			writer.Header().Set("WWW-Authenticate", "Bearer")
			writeError(writer, http.StatusUnauthorized, "authentication required")
			return
		}
		next(writer, request)
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; frame-ancestors 'none'")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(writer, request)
	})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]any{
		"status":  status,
		"message": message,
	})
}
