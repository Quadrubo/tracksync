package server

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Quadrubo/tracksync/server/internal/config"
	_ "github.com/Quadrubo/tracksync/server/internal/converter" // register parsers/serializers
	"github.com/Quadrubo/tracksync/server/internal/target"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// mockTarget implements target.Target for testing.
type mockTarget struct {
	err error
}

func (m *mockTarget) Type() string                                               { return "mock" }
func (m *mockTarget) AcceptedFormats() []string                                  { return []string{"gpx_1.1", "geojson"} }
func (m *mockTarget) Send(_ context.Context, filename string, data []byte) error { return m.err }

type countTarget struct {
	count *int
}

func (c *countTarget) Type() string              { return "count" }
func (c *countTarget) AcceptedFormats() []string { return []string{"gpx_1.1", "geojson"} }
func (c *countTarget) Send(_ context.Context, filename string, data []byte) error {
	*c.count++
	return nil
}

func setupTestServer(t *testing.T, targetErr error) *Server {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	require.NoError(t, InitDB(db))
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{
		MaxUploadSize: 32 << 20,
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

func uploadRequest(token, deviceID, sourceFormat, filename string, body []byte) *http.Request {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", filename)
	_, _ = part.Write(body)
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if deviceID != "" {
		req.Header.Set("X-Device-ID", deviceID)
	}
	if sourceFormat != "" {
		req.Header.Set("X-Source-Format", sourceFormat)
	}
	return req
}

func serveUpload(srv *Server, token, deviceID, sourceFormat, filename string, body []byte) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, uploadRequest(token, deviceID, sourceFormat, filename, body))
	return rec
}

var validGPX = []byte(`<gpx version="1.1"><trk><trkseg><trkpt lat="1" lon="2"/></trkseg></trk></gpx>`)

func TestHealth(t *testing.T) {
	srv := setupTestServer(t, nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/health", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUpload_NoAuth(t *testing.T) {
	srv := setupTestServer(t, nil)
	rec := serveUpload(srv, "", "dev-1", "gpx_1.1", "track.gpx", validGPX)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUpload_BadToken(t *testing.T) {
	srv := setupTestServer(t, nil)
	rec := serveUpload(srv, "wrong-token", "dev-1", "gpx_1.1", "track.gpx", validGPX)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUpload_MissingDeviceID(t *testing.T) {
	srv := setupTestServer(t, nil)
	rec := serveUpload(srv, "valid-token", "", "gpx_1.1", "track.gpx", validGPX)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUpload_ForbiddenDevice(t *testing.T) {
	srv := setupTestServer(t, nil)
	rec := serveUpload(srv, "limited-token", "dev-1", "gpx_1.1", "track.gpx", validGPX)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestUpload_Success(t *testing.T) {
	srv := setupTestServer(t, nil)
	rec := serveUpload(srv, "valid-token", "dev-1", "gpx_1.1", "track.gpx", validGPX)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestUpload_Duplicate(t *testing.T) {
	srv := setupTestServer(t, nil)

	rec1 := serveUpload(srv, "valid-token", "dev-1", "gpx_1.1", "track.gpx", validGPX)
	require.Equal(t, http.StatusCreated, rec1.Code)

	rec2 := serveUpload(srv, "valid-token", "dev-1", "gpx_1.1", "track.gpx", validGPX)
	assert.Equal(t, http.StatusOK, rec2.Code)
}

func TestUpload_TargetFailure(t *testing.T) {
	srv := setupTestServer(t, fmt.Errorf("connection refused"))
	rec := serveUpload(srv, "valid-token", "dev-1", "gpx_1.1", "track.gpx", validGPX)
	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestUpload_RetryAfterTargetFailure(t *testing.T) {
	mock := &mockTarget{err: fmt.Errorf("connection refused")}
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	require.NoError(t, InitDB(db))
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{
		Clients: []config.Client{
			{ID: "test-client", Token: "valid-token", AllowedDeviceIDs: []string{"dev-1"}},
		},
	}
	srv := New(cfg, db, map[string]target.Target{"dev-1": mock})

	// First attempt fails
	rec1 := serveUpload(srv, "valid-token", "dev-1", "gpx_1.1", "track.gpx", validGPX)
	require.Equal(t, http.StatusBadGateway, rec1.Code)

	// Fix the target and retry - should succeed, not be treated as duplicate
	mock.err = nil
	rec2 := serveUpload(srv, "valid-token", "dev-1", "gpx_1.1", "track.gpx", validGPX)
	assert.Equal(t, http.StatusCreated, rec2.Code)
}

func TestUpload_NoTargetForDevice(t *testing.T) {
	srv := setupTestServer(t, nil)
	rec := serveUpload(srv, "limited-token", "dev-2", "gpx_1.1", "track.gpx", validGPX)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUpload_DifferentFilesSameDevice(t *testing.T) {
	srv := setupTestServer(t, nil)

	gpxA := []byte(`<gpx version="1.1"><trk><trkseg><trkpt lat="1" lon="2"/></trkseg></trk></gpx>`)
	gpxB := []byte(`<gpx version="1.1"><trk><trkseg><trkpt lat="3" lon="4"/></trkseg></trk></gpx>`)

	rec1 := serveUpload(srv, "valid-token", "dev-1", "gpx_1.1", "a.gpx", gpxA)
	require.Equal(t, http.StatusCreated, rec1.Code)

	rec2 := serveUpload(srv, "valid-token", "dev-1", "gpx_1.1", "b.gpx", gpxB)
	assert.Equal(t, http.StatusCreated, rec2.Code)
}

func TestUpload_TargetNotForwarded_OnDuplicate(t *testing.T) {
	calls := 0
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	require.NoError(t, InitDB(db))
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{
		Clients: []config.Client{{ID: "c", Token: "tok", AllowedDeviceIDs: []string{"dev"}}},
	}
	srv := New(cfg, db, map[string]target.Target{"dev": &countTarget{count: &calls}})

	serveUpload(srv, "tok", "dev", "gpx_1.1", "a.gpx", validGPX)
	serveUpload(srv, "tok", "dev", "gpx_1.1", "a.gpx", validGPX)

	assert.Equal(t, 1, calls, "target.Send should only be called once for duplicate content")
}

func TestUpload_SplitFilesMode_SendsMultiple(t *testing.T) {
	calls := 0
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	require.NoError(t, InitDB(db))
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{
		Accounts: []config.Account{{DeviceID: "dev", Markers: []string{"C:split"}, SplitMarkerPosition: "start", SplitMode: "files"}},
		Clients:  []config.Client{{ID: "c", Token: "tok", AllowedDeviceIDs: []string{"dev"}}},
	}
	srv := New(cfg, db, map[string]target.Target{"dev": &countTarget{count: &calls}})

	csvData := []byte("INDEX,TAG,DATE,TIME,LATITUDE N/S,LONGITUDE E/W,HEIGHT,SPEED,HEADING\n" +
		"1,T,260417,110529,52.0N,13.0E,38,1.4,333\n" +
		"2,C,260417,110530,52.1N,13.1E,38,30.0,9\n" +
		"3,T,260417,110531,52.2N,13.2E,38,30.0,13\n")

	rec := serveUpload(srv, "tok", "dev", "columbus-csv", "track.csv", csvData)
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, 2, calls, "files mode should forward one file per split track")
	assert.Equal(t, "uploaded 2/2 files\n", rec.Body.String(), "response reports the multi-file count")
	assert.Equal(t, "2", rec.Header().Get("X-Tracksync-Forwarded-Files"), "fan-out count is reported to the client")
}

// flakyTarget records every filename it accepts and fails sends for any
// filename present in failOn.
type flakyTarget struct {
	sends  []string
	failOn map[string]bool
}

func (f *flakyTarget) Type() string              { return "flaky" }
func (f *flakyTarget) AcceptedFormats() []string { return []string{"gpx_1.1"} }
func (f *flakyTarget) Send(_ context.Context, filename string, _ []byte) error {
	if f.failOn[filename] {
		return fmt.Errorf("simulated failure for %s", filename)
	}
	f.sends = append(f.sends, filename)
	return nil
}

func TestUpload_SplitFilesMode_PartialFailureResendsOnlyMissing(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	require.NoError(t, InitDB(db))
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{
		Accounts: []config.Account{{DeviceID: "dev", Markers: []string{"C:split"}, SplitMarkerPosition: "start", SplitMode: "files"}},
		Clients:  []config.Client{{ID: "c", Token: "tok", AllowedDeviceIDs: []string{"dev"}}},
	}
	tgt := &flakyTarget{failOn: map[string]bool{"track-2.gpx": true}}
	srv := New(cfg, db, map[string]target.Target{"dev": tgt})

	csvData := []byte("INDEX,TAG,DATE,TIME,LATITUDE N/S,LONGITUDE E/W,HEIGHT,SPEED,HEADING\n" +
		"1,T,260417,110529,52.0N,13.0E,38,1.4,333\n" +
		"2,C,260417,110530,52.1N,13.1E,38,30.0,9\n" +
		"3,T,260417,110531,52.2N,13.2E,38,30.0,13\n")

	// First attempt: track-1 delivered, track-2 fails -> whole upload errors.
	rec1 := serveUpload(srv, "tok", "dev", "columbus-csv", "track.csv", csvData)
	require.Equal(t, http.StatusBadGateway, rec1.Code)
	assert.Equal(t, []string{"track-1.gpx"}, tgt.sends, "only the first file is delivered before the failure")

	// Recover the target and retry the identical upload.
	tgt.failOn = nil
	rec2 := serveUpload(srv, "tok", "dev", "columbus-csv", "track.csv", csvData)
	require.Equal(t, http.StatusCreated, rec2.Code)
	assert.Equal(t, []string{"track-1.gpx", "track-2.gpx"}, tgt.sends, "retry resends only the missing file, no duplicate of track-1")
	assert.Equal(t, "uploaded 1/2 files\n", rec2.Body.String(), "retry reports one file sent, the other a duplicate")
}

func TestUpload_MissingSourceFormat(t *testing.T) {
	srv := setupTestServer(t, nil)
	rec := serveUpload(srv, "valid-token", "dev-1", "", "track.gpx", validGPX)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUpload_ConversionFailure(t *testing.T) {
	srv := setupTestServer(t, nil)
	rec := serveUpload(srv, "valid-token", "dev-1", "unknown", "track.bin", []byte("binary data"))
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestUpload_CSVToGeoJSON(t *testing.T) {
	srv := setupTestServer(t, nil)
	csvData := []byte("INDEX,TAG,DATE,TIME,LATITUDE N/S,LONGITUDE E/W,HEIGHT,SPEED,HEADING\n1,T,260417,110529,52.5249440N,13.3693610E,38,1.4,333\n")
	rec := serveUpload(srv, "valid-token", "dev-1", "columbus-csv", "track.csv", csvData)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestUpload_MissingFileField(t *testing.T) {
	srv := setupTestServer(t, nil)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("other", "value")
	_ = writer.Close()

	req := httptest.NewRequest("POST", "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("X-Device-ID", "dev-1")
	req.Header.Set("X-Source-Format", "gpx_1.1")

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
