package dawarich

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Quadrubo/tracksync/server/internal/target"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSend_Success(t *testing.T) {
	var gotAuth, gotContentType, gotBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	d := &Dawarich{
		cfg:    target.Config{URL: ts.URL, APIKey: "test-key"},
		client: &http.Client{Timeout: 5 * time.Second},
	}

	require.NoError(t, d.Send("track.gpx", []byte("<gpx>data</gpx>")))
	assert.Equal(t, "Bearer test-key", gotAuth)
	assert.True(t, strings.HasPrefix(gotContentType, "multipart/form-data"))
	assert.Contains(t, gotBody, "track.gpx")
	assert.Contains(t, gotBody, "<gpx>data</gpx>")
}

func TestSend_PostsToImportsEndpoint(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	d := &Dawarich{
		cfg:    target.Config{URL: ts.URL, APIKey: "k"},
		client: &http.Client{Timeout: 5 * time.Second},
	}
	d.Send("f.gpx", []byte("data"))
	assert.Equal(t, "/api/v1/imports", gotPath)
}

func TestSend_ErrorOnNon2xx(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("something broke"))
	}))
	defer ts.Close()

	d := &Dawarich{
		cfg:    target.Config{URL: ts.URL, APIKey: "k"},
		client: &http.Client{Timeout: 5 * time.Second},
	}

	err := d.Send("f.gpx", []byte("data"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestReadAPIKey_Inline(t *testing.T) {
	d := &Dawarich{cfg: target.Config{APIKey: "inline-key"}}
	key, err := d.readAPIKey()
	require.NoError(t, err)
	assert.Equal(t, "inline-key", key)
}

func TestReadAPIKey_File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api-key")
	os.WriteFile(path, []byte("  file-key\n"), 0600)

	d := &Dawarich{cfg: target.Config{APIKeyFile: path}}
	key, err := d.readAPIKey()
	require.NoError(t, err)
	assert.Equal(t, "file-key", key)
}

func TestDawarich_Timeout(t *testing.T) {
	tgt, err := target.Get("dawarich", target.Config{URL: "http://localhost", APIKey: "k", Timeout: 42 * time.Second})
	require.NoError(t, err)
	assert.Equal(t, 42*time.Second, tgt.(*Dawarich).client.Timeout)
}

func TestDawarich_DefaultTimeout(t *testing.T) {
	tgt, err := target.Get("dawarich", target.Config{URL: "http://localhost", APIKey: "k"})
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, tgt.(*Dawarich).client.Timeout)
}

func TestDawarich_Type(t *testing.T) {
	assert.Equal(t, "dawarich", (&Dawarich{}).Type())
}
