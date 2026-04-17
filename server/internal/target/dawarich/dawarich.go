package dawarich

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Quadrubo/tracksync/server/internal/target"
)

func init() {
	target.Register("dawarich", func(cfg target.Config) (target.Target, error) {
		if cfg.URL == "" {
			return nil, fmt.Errorf("dawarich: URL is required")
		}
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		return &Dawarich{
			cfg:    cfg,
			client: &http.Client{Timeout: timeout},
		}, nil
	})
}

type Dawarich struct {
	cfg    target.Config
	client *http.Client
}

func (d *Dawarich) Type() string                { return "dawarich" }
func (d *Dawarich) AcceptedFormats() []string    { return []string{"geojson", "gpx_1.1"} }

func (d *Dawarich) Send(ctx context.Context, filename string, data []byte) error {
	apiKey, err := d.readAPIKey()
	if err != nil {
		return fmt.Errorf("reading API key: %w", err)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return fmt.Errorf("creating form file: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return fmt.Errorf("writing file data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("closing multipart writer: %w", err)
	}

	url := d.cfg.URL + "/api/v1/imports"
	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending to dawarich: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("dawarich returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (d *Dawarich) readAPIKey() (string, error) {
	if d.cfg.APIKey != "" {
		return d.cfg.APIKey, nil
	}
	data, err := os.ReadFile(d.cfg.APIKeyFile)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
