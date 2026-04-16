package columbus

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/Quadrubo/tracksync/tracksync/internal/device"
)

func init() {
	device.Register("columbus-p10-pro", func() device.Device { return &P10Pro{} })
}

// P10Pro supports the Columbus P-10 Pro GPS logger.
// Stores track files on a FAT filesystem via USB mass storage.
// Supports GPX and CSV output formats (configured on device).
type P10Pro struct{}

var supportedFormats = []string{"gpx_1.1", "columbus-csv"}

// formatExtensions maps format names to file extensions.
var formatExtensions = map[string]string{
	"gpx_1.1":        ".gpx",
	"columbus-csv": ".csv",
}

func (c *P10Pro) Type() string               { return "columbus-p10-pro" }
func (c *P10Pro) SupportedFormats() []string { return supportedFormats }

func (c *P10Pro) FindFiles(mountPoint string, formats []string) ([]device.FoundFile, error) {
	if len(formats) == 0 {
		formats = supportedFormats
	}

	extToFormat := make(map[string]string, len(formats))
	for _, f := range formats {
		ext, ok := formatExtensions[f]
		if !ok {
			continue
		}
		extToFormat[ext] = f
	}

	var files []device.FoundFile
	err := filepath.WalkDir(mountPoint, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if format, ok := extToFormat[ext]; ok {
			files = append(files, device.FoundFile{Path: path, Format: format})
		}
		return nil
	})
	return files, err
}
