package columbus

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindFiles_GPX(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "track1.gpx"), []byte("<gpx/>"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "track2.gpx"), []byte("<gpx/>"), 0644))

	files, err := (&P10Pro{}).FindFiles(dir)
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestFindFiles_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "lower.gpx"), []byte("<gpx/>"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "UPPER.GPX"), []byte("<gpx/>"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Mixed.Gpx"), []byte("<gpx/>"), 0644))

	files, err := (&P10Pro{}).FindFiles(dir)
	require.NoError(t, err)
	assert.Len(t, files, 3)
}

func TestFindFiles_SkipsNonGPX(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "track.gpx"), []byte("<gpx/>"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "photo.jpg"), []byte("jpeg"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("text"), 0644))

	files, err := (&P10Pro{}).FindFiles(dir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
}

func TestFindFiles_Recursive(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir", "nested")
	require.NoError(t, os.MkdirAll(sub, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "root.gpx"), []byte("<gpx/>"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "nested.gpx"), []byte("<gpx/>"), 0644))

	files, err := (&P10Pro{}).FindFiles(dir)
	require.NoError(t, err)
	require.Len(t, files, 2)

	basenames := map[string]bool{}
	for _, f := range files {
		basenames[filepath.Base(f)] = true
	}
	assert.True(t, basenames["root.gpx"])
	assert.True(t, basenames["nested.gpx"])
}

func TestFindFiles_EmptyDir(t *testing.T) {
	files, err := (&P10Pro{}).FindFiles(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestFindFiles_AbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "track.gpx"), []byte("<gpx/>"), 0644))

	files, err := (&P10Pro{}).FindFiles(dir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.True(t, filepath.IsAbs(files[0]))
}

func TestType(t *testing.T) {
	assert.Equal(t, "columbus-p10-pro", (&P10Pro{}).Type())
}
