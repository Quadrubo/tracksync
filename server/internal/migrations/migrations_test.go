package migrations

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func execFile(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	data, err := files.ReadFile(name)
	require.NoError(t, err)
	_, err = db.Exec(string(data))
	require.NoError(t, err)
}

func TestRun_CreatesNewTables(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, Run(db))

	_, err = db.Exec("SELECT 1 FROM received_files")
	assert.NoError(t, err, "received_files should exist")
	_, err = db.Exec("SELECT 1 FROM forwarded_files")
	assert.NoError(t, err, "forwarded_files should exist")
	_, err = db.Exec("SELECT 1 FROM uploads")
	assert.Error(t, err, "uploads should be dropped")
}

func TestMigration002_ForwardedFilesDedupByFilename(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, Run(db))

	_, err = db.Exec("INSERT INTO received_files (id, device_id, client_id, sha256, filename) VALUES (1, 'dev', 'cli', 'src', 't.csv')")
	require.NoError(t, err)

	insert := func(sha, filename string) int64 {
		res, err := db.Exec("INSERT OR IGNORE INTO forwarded_files (received_file_id, sha256, filename, forwarded_at) VALUES (1, ?, ?, CURRENT_TIMESTAMP)", sha, filename)
		require.NoError(t, err)
		n, _ := res.RowsAffected()
		return n
	}

	// Identical bytes under different filenames both forward (strict per-file).
	assert.Equal(t, int64(1), insert("samehash", "track-1.gpx"))
	assert.Equal(t, int64(1), insert("samehash", "track-2.gpx"))
	// A repeat of the same filename is recognised as already forwarded.
	assert.Equal(t, int64(0), insert("samehash", "track-1.gpx"))
	assert.Equal(t, int64(0), insert("differenthash", "track-1.gpx"), "filename identifies a forwarded file, regardless of content")
}

func TestMigration002_CarriesOverExistingUploads(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	// Simulate a DB already on the old schema with a processed upload.
	execFile(t, db, "001_initial.sql")
	_, err = db.Exec("INSERT INTO uploads (sha256, device_id, client_id, filename) VALUES ('abc', 'dev', 'cli', 't.gpx')")
	require.NoError(t, err)

	execFile(t, db, "002_received_forwarded_files.sql")

	var receivedID int64
	var deviceID, sha string
	var completed bool
	err = db.QueryRow("SELECT id, device_id, sha256, completed_at IS NOT NULL FROM received_files").Scan(&receivedID, &deviceID, &sha, &completed)
	require.NoError(t, err)
	assert.Equal(t, "dev", deviceID)
	assert.Equal(t, "abc", sha)
	assert.True(t, completed, "carried-over upload must be marked completed so it is not re-forwarded")

	// A matching forwarded_files row is reconstructed and linked to the received file.
	var linkedID int64
	var fwdSha, fwdName string
	err = db.QueryRow("SELECT received_file_id, sha256, filename FROM forwarded_files").Scan(&linkedID, &fwdSha, &fwdName)
	require.NoError(t, err)
	assert.Equal(t, receivedID, linkedID, "forwarded file must reference the carried-over received file")
	assert.Equal(t, "abc", fwdSha)
	assert.Equal(t, "t.gpx", fwdName)
}
