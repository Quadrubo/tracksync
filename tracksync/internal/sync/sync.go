package sync

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Quadrubo/tracksync/tracksync/internal/device"
	"github.com/Quadrubo/tracksync/tracksync/internal/migrations"
)

func OpenStateDB(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA journal_mode=wal"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := migrations.Run(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func ClearUploads(db *sql.DB) (int64, error) {
	result, err := db.Exec("DELETE FROM uploaded")
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func AlreadyUploaded(db *sql.DB, hash string) (bool, error) {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM uploaded WHERE sha256 = ?)", hash).Scan(&exists)
	return exists, err
}

func RecordUpload(db *sql.DB, hash, filename, deviceID string) error {
	_, err := db.Exec(
		"INSERT OR IGNORE INTO uploaded (sha256, filename, device_id, uploaded_at) VALUES (?, ?, ?, ?)",
		hash, filename, deviceID, time.Now().UTC(),
	)
	return err
}

func SHA256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func ReadToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// UploadStatus represents the outcome of an upload attempt.
type UploadStatus int

const (
	StatusUploaded  UploadStatus = iota // server accepted as new
	StatusDuplicate                     // server already had this file
)

func Upload(client *http.Client, serverURL, token, deviceID, sourceFormat, filename string, data []byte) (UploadStatus, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return 0, fmt.Errorf("creating form: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return 0, fmt.Errorf("writing form: %w", err)
	}
	if err := writer.Close(); err != nil {
		return 0, fmt.Errorf("closing form: %w", err)
	}

	req, err := http.NewRequest("POST", serverURL+"/upload", &buf)
	if err != nil {
		return 0, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Device-ID", deviceID)
	req.Header.Set("X-Source-Format", sourceFormat)

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	status := strings.TrimSpace(string(body))

	switch resp.StatusCode {
	case http.StatusCreated:
		return StatusUploaded, nil
	case http.StatusOK:
		return StatusDuplicate, nil
	default:
		return 0, fmt.Errorf("server returned %d: %s", resp.StatusCode, status)
	}
}

// Summary holds the results of a sync operation.
type Summary struct {
	Uploaded  int      `json:"uploaded"`
	Duplicate int      `json:"duplicate"`
	Skipped   int      `json:"skipped"`
	Errors    int      `json:"errors"`
	Files     []string `json:"files,omitempty"`
}

// SyncFiles syncs a list of found files to the server.
func SyncFiles(db *sql.DB, client *http.Client, serverURL, token, deviceID string, files []device.FoundFile) Summary {
	var summary Summary

	for _, ff := range files {
		name := filepath.Base(ff.Path)

		data, err := os.ReadFile(ff.Path)
		if err != nil {
			slog.Error("failed to read file", "file", name, "error", err)
			summary.Errors++
			continue
		}

		hash := SHA256Hex(data)

		uploaded, err := AlreadyUploaded(db, hash)
		if err != nil {
			slog.Error("failed to check upload state", "file", name, "error", err)
			summary.Errors++
			continue
		}
		if uploaded {
			slog.Debug("skipped", "file", name, "reason", "already uploaded")
			summary.Skipped++
			continue
		}

		status, err := Upload(client, serverURL, token, deviceID, ff.Format, name, data)
		if err != nil {
			slog.Error("upload failed", "file", name, "error", err)
			summary.Errors++
			continue
		}

		if err := RecordUpload(db, hash, name, deviceID); err != nil {
			slog.Error("failed to record upload", "file", name, "error", err)
			summary.Errors++
			continue
		}

		switch status {
		case StatusUploaded:
			slog.Info("uploaded", "file", name)
			summary.Uploaded++
			summary.Files = append(summary.Files, name)
		case StatusDuplicate:
			slog.Info("duplicate", "file", name, "reason", "already on server")
			summary.Duplicate++
		}
	}

	return summary
}
