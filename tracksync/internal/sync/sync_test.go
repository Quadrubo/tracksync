package sync

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := OpenStateDB(filepath.Join(t.TempDir(), "state.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestAlreadyUploaded_NewHash(t *testing.T) {
	db := openTestDB(t)
	exists, err := AlreadyUploaded(db, "abc123")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestAlreadyUploaded_AfterRecord(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, RecordUpload(db, "abc123", "track.gpx", "dev-1"))

	exists, err := AlreadyUploaded(db, "abc123")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestRecordUpload_Idempotent(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, RecordUpload(db, "hash1", "a.gpx", "dev"))
	require.NoError(t, RecordUpload(db, "hash1", "a.gpx", "dev"), "INSERT OR IGNORE should not error")

	var count int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM uploaded WHERE sha256 = ?", "hash1").Scan(&count))
	assert.Equal(t, 1, count)
}

func TestClearUploads(t *testing.T) {
	db := openTestDB(t)
	for i := 0; i < 5; i++ {
		require.NoError(t, RecordUpload(db, fmt.Sprintf("hash-%d", i), fmt.Sprintf("file-%d.gpx", i), "dev"))
	}

	n, err := ClearUploads(db)
	require.NoError(t, err)
	assert.Equal(t, int64(5), n)

	exists, _ := AlreadyUploaded(db, "hash-0")
	assert.False(t, exists)
}

func TestClearUploads_EmptyDB(t *testing.T) {
	db := openTestDB(t)
	n, err := ClearUploads(db)
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
}

func TestSHA256Hex(t *testing.T) {
	assert.Equal(t,
		"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		SHA256Hex([]byte("hello")),
	)
}

func TestSHA256Hex_DifferentInputs(t *testing.T) {
	assert.NotEqual(t, SHA256Hex([]byte("a")), SHA256Hex([]byte("b")))
}

func TestReadToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(path, []byte("  my-token \n"), 0600))

	tok, err := ReadToken(path)
	require.NoError(t, err)
	assert.Equal(t, "my-token", tok)
}

func TestReadToken_MissingFile(t *testing.T) {
	_, err := ReadToken("/nonexistent/path")
	assert.Error(t, err)
}

func TestUpload_Created(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		assert.Equal(t, "dev-1", r.Header.Get("X-Device-ID"))
		assert.Equal(t, "myhost", r.Header.Get("X-Client-Host"))
		assert.Equal(t, "gpx_1.1", r.Header.Get("X-Source-Format"))
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintln(w, "uploaded")
	}))
	defer ts.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	status, err := Upload(client, ts.URL, "tok", "dev-1", "myhost", "gpx_1.1", "track.gpx", []byte("<gpx/>"))
	require.NoError(t, err)
	assert.Equal(t, "uploaded", status)
}

func TestUpload_Duplicate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "duplicate")
	}))
	defer ts.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	status, err := Upload(client, ts.URL, "tok", "dev", "host", "gpx_1.1", "f.gpx", []byte("data"))
	require.NoError(t, err)
	assert.Equal(t, "duplicate (server)", status)
}

func TestUpload_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintln(w, "internal error")
	}))
	defer ts.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	_, err := Upload(client, ts.URL, "tok", "dev", "host", "gpx_1.1", "f.gpx", []byte("data"))
	assert.Error(t, err)
}

func TestOpenStateDB_CreatesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "state.db")
	db, err := OpenStateDB(path)
	require.NoError(t, err)
	_ = db.Close()

	_, err = os.Stat(path)
	assert.NoError(t, err, "database file should exist")
}
