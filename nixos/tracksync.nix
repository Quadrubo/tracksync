flake:
{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.services.tracksync;
  tracksyncPkg = flake.packages.${pkgs.system}.default;

  mkDeviceConfig =
    device:
    let
      serviceName = "tracksync-${device.deviceId}";
    in
    {
      udevRule = ''ACTION=="add", SUBSYSTEM=="block", ENV{DEVTYPE}=="partition", ATTRS{idVendor}=="${device.usbVendorId}", ATTRS{idProduct}=="${device.usbProductId}", RUN+="${pkgs.systemd}/bin/systemctl start --no-block ${serviceName}.service"'';

      service.${serviceName} = {
        description = "Tracksync - ${device.deviceId}";
        serviceConfig = {
          Type = "oneshot";
          TimeoutStartSec = "infinity";
          User = cfg.user;
          ExecStart =
            let
              script = pkgs.writeShellScript serviceName ''
                set -euo pipefail

                export HOME=$(eval echo "~$(id -un)")
                export DBUS_SESSION_BUS_ADDRESS="unix:path=/run/user/$(id -u)/bus"

                ${pkgs.libnotify}/bin/notify-send -i drive-removable-media "Tracksync" "GPS device detected, syncing..." 2>/dev/null || true

                DEV="/dev/disk/by-id/${device.diskById}"

                # Wait for device to be mounted or udisks2 to be ready (up to 60s)
                MOUNT=""
                SELF_MOUNTED=0
                for i in $(seq 1 60); do
                  MOUNT=$(${pkgs.util-linux}/bin/findmnt -n -o TARGET "$DEV" 2>/dev/null || true)
                  [ -n "$MOUNT" ] && break
                  if ${pkgs.udisks2}/bin/udisksctl info -b "$DEV" >/dev/null 2>&1; then
                    ${pkgs.udisks2}/bin/udisksctl mount -b "$DEV" --no-user-interaction 2>&1
                    MOUNT=$(${pkgs.util-linux}/bin/findmnt -n -o TARGET "$DEV" 2>/dev/null || true)
                    SELF_MOUNTED=1
                    break
                  fi
                  sleep 1
                done

                if [ -z "$MOUNT" ]; then
                  echo "Failed to mount $DEV" >&2
                  ${pkgs.libnotify}/bin/notify-send -i dialog-error "Tracksync" "Could not mount GPS device." 2>/dev/null || true
                  exit 1
                fi

                echo "Device mounted at $MOUNT"

                SUMMARY=$(${tracksyncPkg}/bin/tracksync \
                  --server-url "${cfg.serverUrl}" \
                  --token-file "${cfg.tokenFile}" \
                  --device-type "${device.deviceType}" \
                  --device-id "${device.deviceId}" \
                  --mount-point "$MOUNT" \
                  --log-format json \
                  ${lib.optionalString (cfg.stateDB != null) "--state-db \"${cfg.stateDB}\""}) && RC=0 || RC=$?

                UPLOADED=$(echo "$SUMMARY" | ${pkgs.jq}/bin/jq -r '.uploaded // 0')
                SKIPPED=$(echo "$SUMMARY" | ${pkgs.jq}/bin/jq -r '.skipped // 0')
                ERRORS=$(echo "$SUMMARY" | ${pkgs.jq}/bin/jq -r '.errors // 0')

                # Only unmount if we mounted it ourselves
                if [ "$SELF_MOUNTED" = 1 ]; then
                  ${pkgs.udisks2}/bin/udisksctl unmount -b "$DEV" --no-user-interaction 2>/dev/null || true
                fi

                if [ "$RC" = 0 ]; then
                  ${pkgs.libnotify}/bin/notify-send -i emblem-ok "Tracksync" "$UPLOADED uploaded, $SKIPPED skipped" 2>/dev/null || true
                else
                  ${pkgs.libnotify}/bin/notify-send -i dialog-error "Tracksync" "$UPLOADED uploaded, $SKIPPED skipped, $ERRORS failed" 2>/dev/null || true
                  exit 1
                fi
              '';
            in
            "${script}";
        };
      };
    };

  deviceConfigs = map mkDeviceConfig cfg.devices;

in
{
  options.services.tracksync = {

    enable = lib.mkEnableOption "Tracksync GPS logger sync";

    user = lib.mkOption {
      type = lib.types.str;
      description = "User to run the sync service as.";
      example = "julian";
    };

    serverUrl = lib.mkOption {
      type = lib.types.str;
      description = "URL of the tracksync server.";
      example = "https://tracksync.example.com";
    };

    tokenFile = lib.mkOption {
      type = lib.types.str;
      description = "Path to file containing the auth token.";
      example = "/run/secrets/tracksync-token";
    };

    stateDB = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      description = "Path to the client state database. Defaults to ~/.local/share/tracksync/state.db.";
    };

    devices = lib.mkOption {
      type = lib.types.listOf (
        lib.types.submodule {
          options = {
            deviceId = lib.mkOption {
              type = lib.types.str;
              description = "Device identifier (must match server config).";
            };
            deviceType = lib.mkOption {
              type = lib.types.str;
              default = "columbus-p10-pro";
              description = "Device type.";
            };
            usbVendorId = lib.mkOption {
              type = lib.types.str;
              description = "USB vendor ID from lsusb.";
            };
            usbProductId = lib.mkOption {
              type = lib.types.str;
              description = "USB product ID from lsusb.";
            };
            diskById = lib.mkOption {
              type = lib.types.str;
              description = "Partition name in /dev/disk/by-id/ (from ls /dev/disk/by-id/).";
              example = "usb-COLUMBUS_P-10_Pro_054738313533-0:0-part1";
            };
          };
        }
      );
      default = [ ];
      description = "GPS devices to sync when plugged in.";
    };
  };

  config = lib.mkIf cfg.enable {
    services.udev.extraRules = lib.concatMapStringsSep "\n" (d: d.udevRule) deviceConfigs;

    systemd.services = lib.mkMerge (map (d: d.service) deviceConfigs);
  };
}
