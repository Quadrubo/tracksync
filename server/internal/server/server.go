package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Quadrubo/tracksync/server/internal/config"
	"github.com/Quadrubo/tracksync/server/internal/converter"
	"github.com/Quadrubo/tracksync/server/internal/migrations"
	"github.com/Quadrubo/tracksync/server/internal/target"
)

type Server struct {
	cfg     *config.Config
	db      *sql.DB
	targets map[string]target.Target // device ID -> target
}

func New(cfg *config.Config, db *sql.DB, targets map[string]target.Target) *Server {
	return &Server{cfg: cfg, db: db, targets: targets}
}

func InitDB(db *sql.DB) error {
	if _, err := db.Exec("PRAGMA journal_mode=wal"); err != nil {
		return err
	}
	return migrations.Run(db)
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /upload", s.handleUpload)
	return mux
}

func (s *Server) authenticate(r *http.Request) *config.Client {
	auth := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(auth, "Bearer ")
	if !ok || token == "" {
		return nil
	}
	for i := range s.cfg.Clients {
		t, err := s.cfg.Clients[i].ResolveToken()
		if err != nil {
			slog.Warn("cannot read token for client", "client", s.cfg.Clients[i].ID, "error", err)
			continue
		}
		if subtle.ConstantTimeCompare([]byte(t), []byte(token)) == 1 {
			return &s.cfg.Clients[i]
		}
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintln(w, "ok")
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	// Authenticate
	client := s.authenticate(r)
	if client == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Authorise
	deviceID := r.Header.Get("X-Device-ID")
	if deviceID == "" {
		http.Error(w, "missing X-Device-ID header", http.StatusBadRequest)
		return
	}
	if !client.CanUpload(deviceID) {
		slog.Warn("client not authorised for device", "client", client.ID, "device", deviceID)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "failed to read file", http.StatusInternalServerError)
		return
	}

	// Deduplicate
	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	var exists bool
	err = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM uploads WHERE sha256 = ?)", hashHex).Scan(&exists)
	if err != nil {
		slog.Error("database error", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if exists {
		slog.Info("duplicate", "file", header.Filename, "sha256", hashHex[:12], "client", client.ID)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "duplicate")
		return
	}

	// Forward to target
	t, ok := s.targets[deviceID]
	if !ok {
		slog.Error("no target configured", "device", deviceID)
		http.Error(w, "no target for device", http.StatusBadRequest)
		return
	}

	// Convert format if needed
	sourceFormat := r.Header.Get("X-Source-Format")
	if sourceFormat == "" {
		http.Error(w, "missing X-Source-Format header", http.StatusBadRequest)
		return
	}

	convertedData, chosenFormat, newFilename, err := converter.Convert(
		sourceFormat, data, t.AcceptedFormats(), header.Filename, s.cfg.PassthroughConversion,
	)
	if err != nil {
		slog.Error("conversion failed",
			"source", sourceFormat,
			"file", header.Filename,
			"error", err,
		)
		http.Error(w, "format conversion failed", http.StatusUnprocessableEntity)
		return
	}

	if chosenFormat != sourceFormat {
		slog.Info("converted",
			"file", header.Filename,
			"from", sourceFormat,
			"to", chosenFormat,
			"newFile", newFilename,
		)
	}

	if err := t.Send(newFilename, convertedData); err != nil {
		slog.Error("target failed", "target", t.Type(), "source", header.Filename, "file", newFilename, "error", err)
		http.Error(w, "target forward failed", http.StatusBadGateway)
		return
	}

	// Record successful upload
	result, err := s.db.Exec(
		"INSERT OR IGNORE INTO uploads (sha256, device_id, client_id, filename, uploaded_at) VALUES (?, ?, ?, ?, ?)",
		hashHex, deviceID, client.ID, header.Filename, time.Now().UTC(),
	)
	if err != nil {
		slog.Error("failed to record upload", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		slog.Info("duplicate (concurrent)", "source", header.Filename, "file", newFilename, "sha256", hashHex[:12], "client", client.ID)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "duplicate")
		return
	}

	slog.Info("uploaded", "source", header.Filename, "file", newFilename, "sha256", hashHex[:12], "client", client.ID, "device", deviceID)
	w.WriteHeader(http.StatusCreated)
	_, _ = fmt.Fprintln(w, "uploaded")
}
