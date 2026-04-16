package server

import (
	"bytes"
	"database/sql"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Quadrubo/tracksync/server/internal/config"
	"github.com/Quadrubo/tracksync/server/internal/target"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// mockTarget implements target.Target for testing.
type mockTarget struct {
	err error
}

func (m *mockTarget) Type() string                            { return "mock" }
func (m *mockTarget) Send(filename string, data []byte) error { return m.err }

type countTarget struct {
	count *int
}

func (c *countTarget) Type() string { return "count" }
func (c *countTarget) Send(filename string, data []byte) error {
	*c.count++
	return nil
}

func setupTestServer(t *testing.T, targetErr error) *Server {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	require.NoError(t, InitDB(db))
	t.Cleanup(func() { db.Close() })

	cfg := &config.Config{
		Clients: []config.Client{
			{ID: "test-client", Token: "valid-token", AllowedDeviceIDs: []string{"dev-1"}},
			{ID: "limited-client", Token: "limited-token", AllowedDeviceIDs: []string{"dev-2"}},
		},
	}
	targets := map[string]target.Target{
		"dev-1": &mockTarget{err: targetErr},
	}

	return New(cfg, db, targets)
}

func uploadRequest(token, deviceID, filename string, body []byte) *http.Request {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", filename)
	part.Write(body)
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if deviceID != "" {
		req.Header.Set("X-Device-ID", deviceID)
	}
	return req
}

func serveUpload(srv *Server, token, deviceID, filename string, body []byte) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, uploadRequest(token, deviceID, filename, body))
	return rec
}

func TestHealth(t *testing.T) {
	srv := setupTestServer(t, nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/health", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUpload_NoAuth(t *testing.T) {
	srv := setupTestServer(t, nil)
	rec := serveUpload(srv, "", "dev-1", "track.gpx", []byte("<gpx/>"))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUpload_BadToken(t *testing.T) {
	srv := setupTestServer(t, nil)
	rec := serveUpload(srv, "wrong-token", "dev-1", "track.gpx", []byte("<gpx/>"))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUpload_MissingDeviceID(t *testing.T) {
	srv := setupTestServer(t, nil)
	rec := serveUpload(srv, "valid-token", "", "track.gpx", []byte("<gpx/>"))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUpload_ForbiddenDevice(t *testing.T) {
	srv := setupTestServer(t, nil)
	rec := serveUpload(srv, "limited-token", "dev-1", "track.gpx", []byte("<gpx/>"))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUpload_Success(t *testing.T) {
	srv := setupTestServer(t, nil)
	rec := serveUpload(srv, "valid-token", "dev-1", "track.gpx", []byte("<gpx>data</gpx>"))
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestUpload_Duplicate(t *testing.T) {
	srv := setupTestServer(t, nil)
	body := []byte("<gpx>duplicate</gpx>")

	rec1 := serveUpload(srv, "valid-token", "dev-1", "track.gpx", body)
	require.Equal(t, http.StatusCreated, rec1.Code)

	rec2 := serveUpload(srv, "valid-token", "dev-1", "track.gpx", body)
	assert.Equal(t, http.StatusOK, rec2.Code)
}

func TestUpload_TargetFailure(t *testing.T) {
	srv := setupTestServer(t, fmt.Errorf("connection refused"))
	rec := serveUpload(srv, "valid-token", "dev-1", "track.gpx", []byte("<gpx>fail</gpx>"))
	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestUpload_NoTargetForDevice(t *testing.T) {
	srv := setupTestServer(t, nil)
	rec := serveUpload(srv, "limited-token", "dev-2", "track.gpx", []byte("<gpx/>"))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUpload_DifferentFilesSameDevice(t *testing.T) {
	srv := setupTestServer(t, nil)

	rec1 := serveUpload(srv, "valid-token", "dev-1", "a.gpx", []byte("file-a"))
	require.Equal(t, http.StatusCreated, rec1.Code)

	rec2 := serveUpload(srv, "valid-token", "dev-1", "b.gpx", []byte("file-b"))
	assert.Equal(t, http.StatusCreated, rec2.Code)
}

func TestUpload_TargetNotForwarded_OnDuplicate(t *testing.T) {
	calls := 0
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	require.NoError(t, InitDB(db))
	t.Cleanup(func() { db.Close() })

	cfg := &config.Config{
		Clients: []config.Client{{ID: "c", Token: "tok", AllowedDeviceIDs: []string{"dev"}}},
	}
	srv := New(cfg, db, map[string]target.Target{"dev": &countTarget{count: &calls}})
	body := []byte("same-content")

	serveUpload(srv, "tok", "dev", "a.gpx", body)
	serveUpload(srv, "tok", "dev", "a.gpx", body)

	assert.Equal(t, 1, calls, "target.Send should only be called once for duplicate content")
}

func TestUpload_MissingFileField(t *testing.T) {
	srv := setupTestServer(t, nil)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("other", "value")
	writer.Close()

	req := httptest.NewRequest("POST", "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("X-Device-ID", "dev-1")

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
