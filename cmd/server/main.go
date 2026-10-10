package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/drona-gyawali/logStore/internal/engine"
	"github.com/drona-gyawali/logStore/internal/state"
	"github.com/drona-gyawali/logStore/internal/storage"
)

type server struct {
	engine *engine.Engine
}

type valueRequest struct {
	Value string `json:"value"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type valueResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
		})
	})

	mux.HandleFunc("PUT /kv/{key}", s.set)
	mux.HandleFunc("GET /kv/{key}", s.get)
	mux.HandleFunc("DELETE /kv/{key}", s.delete)

	return mux
}

func (s *server) set(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "key cannot be empty")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer func() { _ = r.Body.Close() }()

	var request valueRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			"request body must be JSON containing a value",
		)
		return
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(
			w,
			http.StatusBadRequest,
			"request body must contain exactly one JSON object",
		)
		return
	}

	if err := s.engine.Set(r.Context(), key, request.Value); err != nil {
		log.Printf("set key %q: %v", key, err)
		writeError(
			w,
			http.StatusInternalServerError,
			"could not store value",
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *server) get(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "key cannot be empty")
		return
	}

	value, exists, err := s.engine.Get(r.Context(), key)
	if err != nil {
		log.Printf("get key %q: %v", key, err)
		writeError(
			w,
			http.StatusInternalServerError,
			"could not retrieve value",
		)
		return
	}

	if !exists {
		writeError(w, http.StatusNotFound, "key not found")
		return
	}

	writeJSON(w, http.StatusOK, valueResponse{
		Key:   key,
		Value: value,
	})
}

func (s *server) delete(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		writeError(w, http.StatusBadRequest, "key cannot be empty")
		return
	}

	if err := s.engine.Delete(r.Context(), key); err != nil {
		log.Printf("delete key %q: %v", key, err)
		writeError(
			w,
			http.StatusInternalServerError,
			"could not delete value",
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	store, err := storage.NewS3StorageFromEnv(ctx)
	if err != nil {
		log.Fatalf("initialize storage: %v", err)
	}

	app := engine.New(store)
	if err := app.Initialize(ctx); err != nil {
		log.Fatalf("initialize engine: %v", err)
	}
	// TODO: We should probably run the compactor in a separate process, but for now
	// TODO: We have to make the timer more dynamic by including in env with var like COMPACTION_INTERVAL
	bgCompactor := state.NewBackgroundCompactor(store, 5*time.Minute)
	go bgCompactor.Run(ctx)

	addr := strings.TrimSpace(os.Getenv("ANNALES_ADDR"))
	if addr == "" {
		addr = ":8080"
	}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           (&server{engine: app}).routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown: %v", err)
		}
	}()

	log.Printf("Annales listening on %s", addr)

	if err := httpServer.ListenAndServe(); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("HTTP server: %v", err)
	}
}
