package main

import (
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Quadrubo/tracksync/tracksync/internal/device"
	_ "github.com/Quadrubo/tracksync/tracksync/internal/device/columbus"
	"github.com/Quadrubo/tracksync/tracksync/internal/sync"
	_ "modernc.org/sqlite"
)

func main() {
	serverURL := flag.String("server-url", "", "sync server URL (required)")
	tokenFlag := flag.String("token", "", "auth token (inline)")
	tokenFile := flag.String("token-file", "", "path to auth token file")
	deviceType := flag.String("device-type", "", "device type, e.g. columbus-p10-pro (required)")
	deviceFormat := flag.String("device-format", "", "restrict to a specific format, e.g. gpx, csv (optional)")
	deviceID := flag.String("device-id", "", "device identifier (required)")
	mountPoint := flag.String("mount-point", "", "device mount point (required)")
	stateDB := flag.String("state-db", defaultStateDB(), "path to local SQLite state DB")
	logFormat := flag.String("log-format", "text", "log format: text or json")
	timeout := flag.Duration("timeout", 30*time.Second, "HTTP request timeout")
	clear := flag.Bool("clear", false, "clear local upload history and exit")
	flag.Parse()

	var handler slog.Handler
	if *logFormat == "json" {
		handler = slog.NewJSONHandler(os.Stderr, nil)
	} else {
		handler = slog.NewTextHandler(os.Stderr, nil)
	}
	slog.SetDefault(slog.New(handler))

	if *clear {
		db, err := sync.OpenStateDB(*stateDB)
		if err != nil {
			slog.Error("failed to open state db", "error", err)
			os.Exit(1)
		}
		defer func() { _ = db.Close() }()
		n, err := sync.ClearUploads(db)
		if err != nil {
			slog.Error("failed to clear uploads", "error", err)
			os.Exit(1)
		}
		slog.Info("cleared upload history", "removed", n)
		return
	}

	if *serverURL == "" || *deviceType == "" || *deviceID == "" || *mountPoint == "" {
		flag.Usage()
		os.Exit(1)
	}

	var token string
	switch {
	case *tokenFlag != "":
		token = *tokenFlag
	case *tokenFile != "":
		var err error
		token, err = sync.ReadToken(*tokenFile)
		if err != nil {
			slog.Error("failed to read token", "error", err)
			os.Exit(1)
		}
	default:
		slog.Error("one of --token or --token-file is required")
		os.Exit(1)
	}

	db, err := sync.OpenStateDB(*stateDB)
	if err != nil {
		slog.Error("failed to open state db", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	dev, ok := device.Get(*deviceType)
	if !ok {
		slog.Error("unknown device type", "type", *deviceType, "registered", device.RegisteredTypes())
		os.Exit(1)
	}

	// Determine which formats to search for
	var formats []string
	if *deviceFormat != "" {
		supported := dev.SupportedFormats()
		formatValid := false
		for _, f := range supported {
			if f == *deviceFormat {
				formatValid = true
				break
			}
		}
		if !formatValid {
			slog.Error("unsupported format for device",
				"format", *deviceFormat,
				"device", *deviceType,
				"supported", supported,
			)
			os.Exit(1)
		}
		formats = []string{*deviceFormat}
	}
	// formats is nil when --device-format not specified → find all supported

	files, err := dev.FindFiles(*mountPoint, formats)
	if err != nil {
		slog.Error("failed to find files", "error", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		slog.Info("no files found", "device", *deviceID)
		writeSummary(sync.Summary{})
		return
	}

	hostname, _ := os.Hostname()
	httpClient := &http.Client{Timeout: *timeout}
	slog.Info("starting sync",
		"device", *deviceID,
		"type", dev.Type(),
		"files", len(files),
	)

	summary := sync.SyncFiles(db, httpClient, *serverURL, token, *deviceID, hostname, files)

	slog.Info("sync complete",
		"uploaded", summary.Uploaded,
		"duplicate", summary.Duplicate,
		"skipped", summary.Skipped,
		"errors", summary.Errors,
	)

	writeSummary(summary)

	if summary.Errors > 0 {
		os.Exit(1)
	}
}

// writeSummary outputs the summary as JSON to stdout for machine consumption.
func writeSummary(s sync.Summary) {
	_ = json.NewEncoder(os.Stdout).Encode(s)
}

func defaultStateDB() string {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataDir, "tracksync", "state.db")
}
