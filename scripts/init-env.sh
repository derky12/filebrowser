#!/usr/bin/env bash
# Writes .env for this machine: the architecture, the Tailscale address, a
# fresh Syncthing key and the device name. It never overwrites an existing .env.
#
#   scripts/init-env.sh                    # detect everything
#   HOST_IP=100.64.0.7 DATA_DIR=/srv/data scripts/init-env.sh
#
# Values you pass as environment variables win over the detected ones. Review
# the file afterwards (DATA_DIR, ports).
set -euo pipefail
cd "$(dirname "$0")/.."

if [ -e .env ]; then
  echo ".env already exists, not touching it (delete it to start over)" >&2
  exit 1
fi

arch="${ARCH:-}"
if [ -z "$arch" ]; then
  case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 ;;
    aarch64 | arm64) arch=arm64 ;;
    *) echo "unknown architecture $(uname -m), set ARCH=amd64 or ARCH=arm64" >&2; exit 1 ;;
  esac
fi

host_ip="${HOST_IP:-}"
if [ -z "$host_ip" ] && command -v tailscale > /dev/null 2>&1; then
  host_ip="$(tailscale ip -4 2> /dev/null | head -n 1 || true)"
fi
if [ -z "$host_ip" ]; then
  echo "could not find the Tailscale address, set HOST_IP=100.x.y.z" >&2
  exit 1
fi

key="$(openssl rand -hex 24)"
name="${SYNCTHING_NAME:-$(hostname -s)}"

# Copy the example and fill in the values, so the comments stay.
sed -e "s|^ARCH=.*|ARCH=$arch|" \
    -e "s|^HOST_IP=.*|HOST_IP=$host_ip|" \
    -e "s|^SYNCTHING_API_KEY=.*|SYNCTHING_API_KEY=$key|" \
    -e "s|^SYNCTHING_NAME=.*|SYNCTHING_NAME=$name|" \
    -e "s|^DATA_DIR=.*|DATA_DIR=${DATA_DIR:-/root}|" \
    .env.example > .env
chmod 600 .env

echo "Wrote .env: ARCH=$arch HOST_IP=$host_ip SYNCTHING_NAME=$name DATA_DIR=${DATA_DIR:-/root}"
echo "Start with: docker compose -f docker-compose.yml up -d"
