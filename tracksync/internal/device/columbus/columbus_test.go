package columbus

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Quadrubo/tracksync/tracksync/internal/device"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindFiles_GPXOnly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "track1.gpx"), []byte("<gpx/>"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "track2.gpx"), []byte("<gpx/>"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.csv"), []byte("csv"), 0644))

	files, err := (&P10Pro{}).FindFiles(dir, []string{"gpx_1.1"})
	require.NoError(t, err)
	assert.Len(t, files, 2)
	for _, f := range files {
		assert.Equal(t, "gpx_1.1", f.Format)
	}
}

func TestFindFiles_CSVOnly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "track1.csv"), []byte("data"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "track.gpx"), []byte("<gpx/>"), 0644))

	files, err := (&P10Pro{}).FindFiles(dir, []string{"columbus-csv"})
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, "columbus-csv", files[0].Format)
}

func TestFindFiles_AllFormats(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "track.gpx"), []byte("<gpx/>"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.csv"), []byte("csv"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "photo.jpg"), []byte("jpeg"), 0644))

	files, err := (&P10Pro{}).FindFiles(dir, nil)
	require.NoError(t, err)
	assert.Len(t, files, 2)

	formats := map[string]bool{}
	for _, f := range files {
		formats[f.Format] = true
	}
	assert.True(t, formats["gpx_1.1"])
	assert.True(t, formats["columbus-csv"])
}

func TestFindFiles_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "lower.gpx"), []byte("<gpx/>"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "UPPER.GPX"), []byte("<gpx/>"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Mixed.Gpx"), []byte("<gpx/>"), 0644))

	files, err := (&P10Pro{}).FindFiles(dir, []string{"gpx_1.1"})
	require.NoError(t, err)
	assert.Len(t, files, 3)
}

func TestFindFiles_SkipsUnsupported(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "track.gpx"), []byte("<gpx/>"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "photo.jpg"), []byte("jpeg"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("text"), 0644))

	files, err := (&P10Pro{}).FindFiles(dir, nil)
	require.NoError(t, err)
	assert.Len(t, files, 1)
}

func TestFindFiles_Recursive(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir", "nested")
	require.NoError(t, os.MkdirAll(sub, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "root.gpx"), []byte("<gpx/>"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "nested.csv"), []byte("csv"), 0644))

	files, err := (&P10Pro{}).FindFiles(dir, nil)
	require.NoError(t, err)
	assert.Len(t, files, 2)

	basenames := map[string]bool{}
	for _, f := range files {
		basenames[filepath.Base(f.Path)] = true
	}
	assert.True(t, basenames["root.gpx"])
	assert.True(t, basenames["nested.csv"])
}

func TestFindFiles_EmptyDir(t *testing.T) {
	files, err := (&P10Pro{}).FindFiles(t.TempDir(), nil)
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestFindFiles_AbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "track.gpx"), []byte("<gpx/>"), 0644))

	files, err := (&P10Pro{}).FindFiles(dir, nil)
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.True(t, filepath.IsAbs(files[0].Path))
}

func TestFindFiles_ReturnsFoundFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "track.csv"), []byte("csv"), 0644))

	files, err := (&P10Pro{}).FindFiles(dir, []string{"columbus-csv"})
	require.NoError(t, err)
	require.Len(t, files, 1)

	assert.IsType(t, device.FoundFile{}, files[0])
	assert.Equal(t, "columbus-csv", files[0].Format)
	assert.Contains(t, files[0].Path, "track.csv")
}

func TestType(t *testing.T) {
	assert.Equal(t, "columbus-p10-pro", (&P10Pro{}).Type())
}

func TestSupportedFormats(t *testing.T) {
	formats := (&P10Pro{}).SupportedFormats()
	assert.Equal(t, []string{"gpx_1.1", "columbus-csv"}, formats)
}
