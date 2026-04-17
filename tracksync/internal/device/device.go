package device

// FoundFile represents a file found on a device with its detected format.
type FoundFile struct {
	Path   string
	Format string
}

// Device represents a GPS logging device that stores track files
// on a mountable filesystem.
type Device interface {
	// Type returns the device type identifier (e.g. "columbus-p10-pro").
	Type() string
	// SupportedFormats returns the file formats this device can produce.
	SupportedFormats() []string
	// FindFiles returns GPS track files found under mountPoint.
	// If formats is non-empty, only files matching those formats are returned.
	// If formats is empty, all supported formats are searched.
	FindFiles(mountPoint string, formats []string) ([]FoundFile, error)
}
