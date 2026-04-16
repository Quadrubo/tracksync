package device

// Device represents a GPS logging device that stores track files
// on a mountable filesystem.
type Device interface {
	// Type returns the device type identifier (e.g. "columbus-p10-pro").
	Type() string
	// FindFiles returns absolute paths to GPS track files found under mountPoint.
	FindFiles(mountPoint string) ([]string, error)
}
