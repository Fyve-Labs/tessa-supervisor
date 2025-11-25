# Tessa Daemon (tessad)

A small device-side daemon that boots a Tessa-managed device, connects it to control plane messaging, and manages secure remote access tunnels.

It provides a CLI with two primary flows:
- Device Owner: bootstrap the device using an admin-issued token, then run the daemon.
- Tessa Admin: issue bootstrap tokens and operate devices using the internal admin CLI (tessa), e.g., for remote SSH. See the Roles and Flows section below.

Status: active development. Some commands are placeholders and may change. See TODOs.

## Installation

### Option A: Install prebuilt binary (Linux arm64)

- Quick install to /usr/local/bin:

```bash
curl -fsSL https://raw.githubusercontent.com/Fyve-Labs/tessa-daemon/main/install.sh | sudo sh
```
  
Note: The installer currently validates and installs only the linux/arm64 artifact. Other platforms must build from source.


## Roles and Flows

### 1) Tessa Admin — Issue bootstrap token

- Use the internal admin CLI to generate a token for a device name:
  tessa gen-token -n device-01

- Keep the token secure. Share it out-of-band with the device owner.
- Note: The tessa CLI for Tessa Admin is distributed internally and is not part of this repo.

### 2) Device Owner — Bootstrap device once

- Use the token provided by the admin to bootstrap and create the device config and credentials:
  tessad up --token <token> --device-name device-01

Commonly used flags for tessad up:

* -c, --config string     Path to write the config file (default: /etc/tessa/config.yaml)
* -n, --device-name       Device name (required)
* -t, --token string      Token produced by admin tessa CLI (required)

* ""  --data string       Data directory for credentials (default: /var/lib/tessa)
* ""  --server-url string Bootstrap API URL (default: https://device-api.fyve.dev)
* -f, --force             Overwrite existing config if present

What it does:
- Requests and saves device credentials under <data>/credentials:
  - device.crt, device.key, root.crt
- Writes tessad config to the provided -c path.

### 3) Device Owner — Start daemon on the device

- Start and keep it running until interrupted (Ctrl+C) or signaled:
  tessad start -c /etc/tessa/config.yaml

Behavior:
- Connects to the Tessa control plane (NATS) with TLS client auth using the saved credentials.
- Initializes the tunnel manager; dynamic tunnels are managed by remote commands from the control plane.
- Graceful shutdown on SIGINT/SIGTERM.

### 4) Tessa Admin — Remote access

- From the admin network, use the internal CLI to open a secure SSH session:
  tessa ssh device-01

Note: Exact admin-side capabilities are governed by the internal control-plane and admin tooling. The daemon exposes no direct public API.


## CLI Reference (device-side)

Global:
- -c, --config string  Path to config file (default: /etc/tessa/config.yaml)

Commands:
- tessad up       Bootstrap device using a token (writes config and credentials)
- tessad start    Start the daemon (connects to control plane and manages tunnels)
- tessad check    Validate configuration (Not yet implemented)
- tessad update   Self-update (Not yet implemented)


## Configuration

Default config file path: /etc/tessa/config.yaml (overridable with -c/--config).

## Environment Variables
- TESSA_NATS_URL
  - Overrides the control-plane NATS server URL.
  - Example: tls://nats.example.com:4222

- TESSA_TUNNEL_SERVER_ADDR
  - Overrides the tunnel server address used by the FRP client.
  - Example: tunnel.example.com

 
## Development

- Live reload during development:
  
```bash
air
```

- Run with a local config file:

```bash
./tessad start -c config.yaml
```

## License
- TODO: Add license information and LICENSE file.

## TODOs
- Implement self-update.
- Implement config validation.
- Implement more remote commands (e.g., report logs, metrics, reboot, shutdown, etc.).
