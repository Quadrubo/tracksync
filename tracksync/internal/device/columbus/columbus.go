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
// Stores GPX 1.1 files on a FAT filesystem via USB mass storage.
type P10Pro struct{}

func (c *P10Pro) Type() string { return "columbus-p10-pro" }

func (c *P10Pro) FindFiles(mountPoint string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(mountPoint, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.IsDir() && strings.EqualFold(filepath.Ext(path), ".gpx") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
