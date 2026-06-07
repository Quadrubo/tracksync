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
	"strconv"
	"strings"
	"time"

	"github.com/Quadrubo/tracksync/server/internal/config"
	"github.com/Quadrubo/tracksync/server/internal/converter"
	"github.com/Quadrubo/tracksync/server/internal/migrations"
	"github.com/Quadrubo/tracksync/server/internal/target"
)

type Server struct {
	cfg        *config.Config
	db         *sql.DB
	targets    map[string]target.Target           // device ID -> target
	markerOpts map[string]converter.MarkerOptions // device ID -> marker config
}

func New(cfg *config.Config, db *sql.DB, targets map[string]target.Target) *Server {
	markerOpts := make(map[string]converter.MarkerOptions, len(cfg.Accounts))
	for _, a := range cfg.Accounts {
		// Validated at config load.
		rules, _ := converter.ParseMarkerRules(a.Markers)
		markerOpts[a.DeviceID] = converter.MarkerOptions{
			Rules:               rules,
			SplitMarkerPosition: a.SplitMarkerPosition,
			SplitMode:           a.SplitMode,
		}
	}
	return &Server{cfg: cfg, db: db, targets: targets, markerOpts: markerOpts}
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
		if subtle.ConstantTimeCompare([]byte(s.cfg.Clients[i].Token), []byte(token)) == 1 {
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

	// Validate target exists for device
	t, ok := s.targets[deviceID]
	if !ok {
		slog.Error("no target configured", "device", deviceID)
		http.Error(w, "no target for device", http.StatusBadRequest)
		return
	}

	// Validate source format
	sourceFormat := r.Header.Get("X-Source-Format")
	if sourceFormat == "" {
		http.Error(w, "missing X-Source-Format header", http.StatusBadRequest)
		return
	}

	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadSize+1<<20)

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

	// Fast path: skip a fully processed payload without re-converting.
	hash := sha256.Sum256(data)
	hashHex := hex.EncodeToString(hash[:])

	var receivedFileID int64
	var completed bool
	switch err := s.db.QueryRow(
		"SELECT id, completed_at IS NOT NULL FROM received_files WHERE device_id = ? AND sha256 = ?",
		deviceID, hashHex,
	).Scan(&receivedFileID, &completed); {
	case err == sql.ErrNoRows:
		// New payload for this device.
	case err != nil:
		slog.Error("database error", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	case completed:
		slog.Info("duplicate", "file", header.Filename, "sha256", hashHex[:12], "client", client.ID)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "duplicate")
		return
	}

	outputs, err := converter.Convert(
		sourceFormat, data, t.AcceptedFormats(), header.Filename, s.cfg.PassthroughConversion, s.markerOpts[deviceID],
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
	if outputs[0].Format != sourceFormat {
		slog.Info("converted",
			"file", header.Filename,
			"from", sourceFormat,
			"to", outputs[0].Format,
			"files", len(outputs),
		)
	}

	// Reuse the row from an earlier incomplete attempt, or claim a new one.
	if receivedFileID == 0 {
		receivedFileID, err = s.claimReceivedFile(deviceID, client.ID, hashHex, header.Filename)
		if err != nil {
			slog.Error("database error", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
	}

	// Claim each file before sending and commit only after; a retry forwards
	// only the files still missing.
	sent := 0
	skipped := 0
	for _, out := range outputs {
		outHash := sha256.Sum256(out.Data)
		outHashHex := hex.EncodeToString(outHash[:])

		tx, err := s.db.Begin()
		if err != nil {
			slog.Error("database error", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		result, err := tx.Exec(
			"INSERT OR IGNORE INTO forwarded_files (received_file_id, sha256, filename, forwarded_at) VALUES (?, ?, ?, ?)",
			receivedFileID, outHashHex, out.Filename, time.Now().UTC(),
		)
		if err != nil {
			_ = tx.Rollback()
			slog.Error("database error", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if rows, _ := result.RowsAffected(); rows == 0 {
			// Already forwarded.
			_ = tx.Rollback()
			skipped++
			continue
		}

		if err := t.Send(r.Context(), out.Filename, out.Data); err != nil {
			_ = tx.Rollback()
			slog.Error("target failed", "target", t.Type(), "source", header.Filename, "file", out.Filename, "error", err)
			http.Error(w, "target forward failed", http.StatusBadGateway)
			return
		}
		if err := tx.Commit(); err != nil {
			slog.Error("failed to commit forwarded file", "file", out.Filename, "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		sent++
	}

	if _, err := s.db.Exec("UPDATE received_files SET completed_at = ? WHERE id = ?", time.Now().UTC(), receivedFileID); err != nil {
		slog.Error("database error", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if sent == 0 {
		// Every output file was already forwarded.
		slog.Info("duplicate", "file", header.Filename, "files", len(outputs), "sha256", hashHex[:12], "client", client.ID)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "duplicate")
		return
	}

	slog.Info("uploaded",
		"source", header.Filename,
		"files", len(outputs),
		"sent", sent,
		"duplicate", skipped,
		"sha256", hashHex[:12],
		"client", client.ID,
		"device", deviceID,
	)
	w.Header().Set("X-Tracksync-Forwarded-Files", strconv.Itoa(sent))
	w.WriteHeader(http.StatusCreated)
	_, _ = fmt.Fprintf(w, "uploaded %d/%d files\n", sent, len(outputs))
}

// claimReceivedFile inserts the payload row if absent and returns its id,
// returning the existing id on a concurrent insert of the same (device, payload).
func (s *Server) claimReceivedFile(deviceID, clientID, sha256Hex, filename string) (int64, error) {
	res, err := s.db.Exec(
		"INSERT OR IGNORE INTO received_files (device_id, client_id, sha256, filename, received_at) VALUES (?, ?, ?, ?, ?)",
		deviceID, clientID, sha256Hex, filename, time.Now().UTC(),
	)
	if err != nil {
		return 0, err
	}
	if rows, _ := res.RowsAffected(); rows > 0 {
		return res.LastInsertId()
	}
	var id int64
	err = s.db.QueryRow("SELECT id FROM received_files WHERE device_id = ? AND sha256 = ?", deviceID, sha256Hex).Scan(&id)
	return id, err
}
