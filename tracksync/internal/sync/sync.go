package sync

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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
		db.Close()
		return nil, err
	}
	if err := migrations.Run(db); err != nil {
		db.Close()
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

func Upload(client *http.Client, serverURL, token, deviceID, hostname, filename string, data []byte) (string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", fmt.Errorf("creating form: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return "", fmt.Errorf("writing form: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("closing form: %w", err)
	}

	req, err := http.NewRequest("POST", serverURL+"/upload", &buf)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Device-ID", deviceID)
	req.Header.Set("X-Client-Host", hostname)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	status := strings.TrimSpace(string(body))

	switch resp.StatusCode {
	case http.StatusCreated:
		return "uploaded", nil
	case http.StatusOK:
		return "duplicate (server)", nil
	default:
		return "", fmt.Errorf("server returned %d: %s", resp.StatusCode, status)
	}
}
