# Tracksync

Syncs track files from GPS loggers to self-hosted location tracking services like [Dawarich](https://dawarich.app/). Plug in your device, tracks get uploaded automatically.

## Features

- Format conversion - automatically converts between device and target formats to preserve all data
- Deduplication - files are only uploaded once
- Multi-client - sync from multiple machines to the same server
- Extensible [device support](#supported-devices)
- Extensible [target support](#supported-targets)
- Per-client auth tokens

### NixOS module

- Automatic sync on USB plug-in
- Desktop notifications on sync progress
- Mounts device automatically via udisks2
- Integrates with sops-nix for secret management

## How it works

Tracksync consists of three parts:

- **Client** - a CLI tool that reads track files from a mounted GPS device and uploads them to the server.
- **Server** - receives uploads, deduplicates them, converts them to the best format for the target, and forwards files to a configured target like Dawarich. Runs as a Docker container.
- **NixOS module** - automates the client by detecting USB plug-in via udev, mounting the device, running the sync, and sending desktop notifications.

## Install the client

### Go

```bash
go install github.com/Quadrubo/tracksync/tracksync@latest
```

### Nix

```bash
nix run github:Quadrubo/tracksync
```

## Configure the NixOS Module

You can use a [NixOS module](#nixos-module) to automate the complete process.

## Run the server

### Docker Compose

```yaml
services:
  tracksync-server:
    image: ghcr.io/quadrubo/tracksync/server:latest
    ports:
      - "8080:8080"
    environment:
      - ACCOUNT__0__DEVICE_ID=my-columbus
      - ACCOUNT__0__TARGET_URL=http://dawarich:3000
      - ACCOUNT__0__API_KEY=your-api-key
      - CLIENT__0__ID=my-laptop
      - CLIENT__0__TOKEN=your-client-token
      - CLIENT__0__ALLOWED_DEVICES=my-columbus
    volumes:
      - data:/app/data

volumes:
  data:
```

## Server configuration

All configuration is done via environment variables.

| Variable                     | Default         | Required                | Description                        |
| ---------------------------- | --------------- | ----------------------- | ---------------------------------- |
| `PORT`                       | `8080`          | No                      | Server port                        |
| `STATE_DB`                   | `data/state.db` | No                      | SQLite database path               |
| `TARGET_TIMEOUT`             | `30s`           | No                      | HTTP timeout for target requests   |
| `ACCOUNT__N__DEVICE_ID`      |                 | Yes                     | Device identifier                  |
| `ACCOUNT__N__TARGET_TYPE`    | `dawarich`      | No                      | Target type                        |
| `ACCOUNT__N__TARGET_URL`     |                 | Yes                     | Target instance URL                |
| `ACCOUNT__N__API_KEY`        |                 | Yes if not API_KEY_FILE | API key (inline)                   |
| `ACCOUNT__N__API_KEY_FILE`   |                 | Yes if not API_KEY      | API key (file path)                |
| `CLIENT__N__ID`              |                 | Yes                     | Client identifier                  |
| `CLIENT__N__TOKEN`           |                 | Yes if not TOKEN_FILE   | Auth token (inline)                |
| `CLIENT__N__TOKEN_FILE`      |                 | Yes if not TOKEN        | Auth token (file path)             |
| `CLIENT__N__ALLOWED_DEVICES` |                 | Yes                     | Comma-separated allowed device IDs |

\* One of the inline or file variant is required.

Replace `N` with `0`, `1`, `2`, etc. for multiple accounts/clients.

## Client usage

```bash
tracksync \
  --server-url http://localhost:8080 \
  --token your-client-token \
  --device-type columbus-p10-pro \
  --device-id my-columbus \
  --mount-point /mnt/gps
```

By default, the client picks up all file formats the device supports. Use `--device-format` to restrict to a specific one (e.g. `--device-format columbus-csv`).

| Flag              | Default                             | Required              | Description                          |
| ----------------- | ----------------------------------- | --------------------- | ------------------------------------ |
| `--server-url`    |                                     | Yes                   | Tracksync server URL                 |
| `--token`         |                                     | Yes if not token-file | Auth token inline                    |
| `--token-file`    |                                     | Yes if not token      | Auth token from file                 |
| `--device-type`   |                                     | Yes                   | GPS device type                      |
| `--device-format` |                                     | No                    | Restrict to a specific device format |
| `--device-id`     |                                     | Yes                   | Device identifier                    |
| `--mount-point`   |                                     | Yes                   | Device mount point                   |
| `--state-db`      | `~/.local/share/tracksync/state.db` | No                    | Client state database                |
| `--log-format`    | `text`                              | No                    | Log format: text, json               |
| `--timeout`       | `30s`                               | No                    | HTTP request timeout                 |
| `--clear`         | `false`                             | No                    | Clear upload history                 |

## API

| Endpoint  | Method | Auth         | Description                                   |
| --------- | ------ | ------------ | --------------------------------------------- |
| `/health` | GET    | No           | Liveness check                                |
| `/upload` | POST   | Bearer token | Upload a track file (multipart, field `file`) |

The `/upload` endpoint requires `X-Device-ID` and `X-Source-Format` headers.

## NixOS module

Add tracksync as a flake input:

```nix
# flake.nix
inputs.tracksync.url = "github:Quadrubo/tracksync";
```

Import the module and configure:

```nix
{
  imports = [ inputs.tracksync.nixosModules.default ];

  services.tracksync = {
    enable = true;
    user = "your-username";
    serverUrl = "https://tracksync.example.com";
    tokenFile = "/run/secrets/tracksync-token";
    devices = [{
      deviceId = "my-device";
      deviceType = "columbus-p10-pro";
      # deviceFormat = "columbus-csv";  # optional, omit to find all supported formats
      usbVendorId = "xxxx";     # from lsusb
      usbProductId = "xxxx";    # from lsusb
      diskById = "usb-xxxx-part1";  # from ls /dev/disk/by-id/
    }];
  };
}
```

The sync service runs outside of a desktop session, so it needs a polkit rule in order to automatically mount the gps logger device. You can find the device serial with `lsblk --nodeps -o name,serial` while the device is connected.

```nix
{
  security.polkit.extraConfig = ''
    polkit.addRule(function(action, subject) {
      if (action.id == "org.freedesktop.udisks2.filesystem-mount" &&
          subject.user == "your-username" &&
          action.lookup("drive.serial") == "your-device-serial") {
        return polkit.Result.YES;
      }
    });
  '';
}
```

When the device is plugged in, a systemd service automatically mounts it, syncs all track files, and sends a desktop notification with the result.

## Format conversion

The server automatically converts between file formats to preserve the most data. Each device declares the formats it can produce, and each target declares the formats it accepts. The server parses the source file into a universal track model, then serializes to the target format that preserves the most fields.

For example, when a Columbus P-10 Pro is configured to output CSV (which includes speed and heading), the server converts to GeoJSON before forwarding to Dawarich, since GeoJSON can represent these fields while GPX 1.1 cannot.

### Supported formats

| Format         | Type              | Speed | Heading | Elevation | Satellites | DOP |
| -------------- | ----------------- | ----- | ------- | --------- | ---------- | --- |
| `gpx_1.1`      | Parse + Serialize | No    | No      | Yes       | Yes        | Yes |
| `columbus-csv` | Parse             | Yes   | Yes     | Yes       | No         | No  |
| `geojson`      | Serialize         | Yes   | Yes     | Yes       | Yes        | Yes |

## Supported targets

| Type       | Service                           | Accepted formats     |
| ---------- | --------------------------------- | -------------------- |
| `dawarich` | [Dawarich](https://dawarich.app/) | `gpx_1.1`, `geojson` |

Adding a new target requires implementing the `Target` interface in `server/internal/target/`.

## Supported devices

| Type               | Device            | Supported formats         |
| ------------------ | ----------------- | ------------------------- |
| `columbus-p10-pro` | Columbus P-10 Pro | `gpx_1.1`, `columbus-csv` |

Adding a new device type requires implementing the `Device` interface in `tracksync/internal/device/`.

## Development

### Nix

This repository provides a `flake.nix` with a devshell for development.

Enter the repository and run `direnv allow` or use `nix develop` to start the devshell.

### Other OS

Make sure the following is installed:

- Go
- Just

### General

```bash
# Run server locally
just run-server

# Run client
just run-client --server-url http://localhost:8080 --token test-token \
  --device-type columbus-p10-pro --device-id my-columbus --mount-point /mnt/gps

# Update nix vendor hash after changing Go dependencies
just update-vendor-hash
```
