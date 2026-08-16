// Package main runs the first-party OmniLLM-Studio sandbox worker. The worker
// exposes only the authenticated sandbox protocol v2 and executes through the
// platform sandbox.Runtime implementation.
package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ajbergh/omnillm-studio/internal/sandbox"
)

const maxRequestBytes = 2 << 20

func main() {
	token := strings.TrimSpace(os.Getenv("OMNILLM_SANDBOX_TOKEN"))
	if token == "" {
		log.Fatal("OMNILLM_SANDBOX_TOKEN is required")
	}
	bind := strings.TrimSpace(os.Getenv("OMNILLM_SANDBOX_BIND"))
	if bind == "" {
		bind = "127.0.0.1:8090"
	}
	runtime, err := sandbox.NewLocalRuntime(sandbox.LocalRuntimeConfig{
		RootFS:      os.Getenv("OMNILLM_SANDBOX_ROOTFS"),
		ScratchRoot: os.Getenv("OMNILLM_SANDBOX_SCRATCH_ROOT"),
		BwrapPath:   os.Getenv("OMNILLM_SANDBOX_BWRAP"),
		CgroupRoot:  os.Getenv("OMNILLM_SANDBOX_CGROUP_ROOT"),
	})
	if err != nil {
		log.Fatalf("initialize sandbox runtime: %v", err)
	}

	handler := authenticated(token, newHandler(runtime))
	server := &http.Server{
		Addr:              bind,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      70 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		log.Printf("OmniLLM sandbox worker listening on %s (%s)", bind, runtime.Capabilities().Name)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("sandbox worker: %v", err)
		}
	}()
	<-done
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("sandbox worker shutdown: %v", err)
	}
}

func newHandler(runtime sandbox.Runtime) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v2/capabilities", func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(w, http.StatusOK, runtime.Capabilities())
	})
	mux.HandleFunc("POST /v2/sandboxes", func(w http.ResponseWriter, r *http.Request) {
		var request sandbox.RuntimeCreateRequest
		if err := decodeJSON(w, r, &request); err != nil {
			respondError(w, http.StatusBadRequest, err)
			return
		}
		runtimeID, err := runtime.Create(r.Context(), request)
		if err != nil {
			respondError(w, http.StatusUnprocessableEntity, err)
			return
		}
		respondJSON(w, http.StatusCreated, map[string]string{"runtime_id": runtimeID})
	})
	mux.HandleFunc("/v2/sandboxes/", func(w http.ResponseWriter, r *http.Request) {
		handleSession(runtime, w, r)
	})
	return mux
}

func handleSession(runtime sandbox.Runtime, w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/v2/sandboxes/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		respondError(w, http.StatusNotFound, fmt.Errorf("sandbox runtime session not found"))
		return
	}
	runtimeID, err := url.PathUnescape(parts[0])
	if err != nil || strings.Contains(runtimeID, "/") {
		respondError(w, http.StatusBadRequest, fmt.Errorf("invalid runtime session id"))
		return
	}
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch {
	case r.Method == http.MethodPost && action == "exec" && len(parts) == 2:
		var request sandbox.ExecRequest
		if err := decodeJSON(w, r, &request); err != nil {
			respondError(w, http.StatusBadRequest, err)
			return
		}
		result, err := runtime.Exec(r.Context(), runtimeID, request)
		if err != nil {
			respondError(w, http.StatusUnprocessableEntity, err)
			return
		}
		respondJSON(w, http.StatusOK, result)
	case r.Method == http.MethodPost && action == "cancel" && len(parts) == 2:
		var request struct {
			ExecutionID string `json:"execution_id"`
		}
		if err := decodeJSON(w, r, &request); err != nil {
			respondError(w, http.StatusBadRequest, err)
			return
		}
		if err := runtime.Cancel(r.Context(), runtimeID, request.ExecutionID); err != nil {
			respondError(w, http.StatusUnprocessableEntity, err)
			return
		}
		respondJSON(w, http.StatusOK, map[string]bool{"cancelled": true})
	case r.Method == http.MethodGet && action == "status" && len(parts) == 2:
		status, err := runtime.Status(r.Context(), runtimeID)
		if err != nil {
			respondError(w, http.StatusNotFound, err)
			return
		}
		respondJSON(w, http.StatusOK, status)
	case r.Method == http.MethodDelete && action == "" && len(parts) == 1:
		if err := runtime.Destroy(r.Context(), runtimeID); err != nil {
			respondError(w, http.StatusNotFound, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		respondError(w, http.StatusNotFound, fmt.Errorf("sandbox endpoint not found"))
	}
}

func authenticated(token string, next http.Handler) http.Handler {
	expected := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := []byte(r.Header.Get("Authorization"))
		if len(provided) != len(expected) || subtle.ConstantTimeCompare(provided, expected) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			respondError(w, http.StatusUnauthorized, fmt.Errorf("authentication required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid request body: %w", err)
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err == nil {
		return fmt.Errorf("request body must contain one JSON value")
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("invalid trailing request body: %w", err)
	}
	return nil
}

func respondJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func respondError(w http.ResponseWriter, status int, err error) {
	respondJSON(w, status, map[string]string{"error": err.Error()})
}
