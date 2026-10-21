#!/usr/bin/env bash
# Prepare a fresh Ubuntu host (e.g. an Oracle Cloud Always Free VM) and start
# the FinWatch public demo stack. Run from the repository root:
#
#   sudo deploy/demo/setup-host.sh <demo-domain>
#
# <demo-domain> must resolve to this host, e.g. 203-0-113-10.sslip.io.
# Safe to re-run: an existing deploy/demo/demo.env is kept as is.
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: sudo $0 <demo-domain>" >&2
  exit 2
fi
if [[ $EUID -ne 0 ]]; then
  echo "run as root (sudo)" >&2
  exit 1
fi

domain=$1
demo_dir=$(cd "$(dirname "$0")" && pwd)
env_file="$demo_dir/demo.env"

echo "==> Installing Docker Engine and Compose"
apt-get update -q
DEBIAN_FRONTEND=noninteractive apt-get install -y -q docker.io docker-compose-v2 openssl
systemctl enable --now docker

# Oracle's Ubuntu images ship an iptables INPUT chain that rejects everything
# but SSH. Allow HTTP/HTTPS (QUIC too) ahead of that reject and persist it.
echo "==> Opening ports 80/443 in the host firewall"
for rule in "-p tcp --dport 80" "-p tcp --dport 443" "-p udp --dport 443"; do
  # shellcheck disable=SC2086 # rule is intentionally word-split
  iptables -C INPUT $rule -j ACCEPT 2>/dev/null || iptables -I INPUT 1 $rule -j ACCEPT
done
if command -v netfilter-persistent >/dev/null; then
  netfilter-persistent save
fi

if [[ -f $env_file ]]; then
  echo "==> Keeping existing $env_file"
else
  echo "==> Generating $env_file with fresh random secrets"
  umask 077
  cat >"$env_file" <<EOF
DEMO_DOMAIN=$domain
POSTGRES_USER=finwatch
POSTGRES_PASSWORD=$(openssl rand -hex 24)
POSTGRES_DB=finwatch
JWT_SIGNING_SECRET=$(openssl rand -hex 32)
JWT_ACCESS_TOKEN_TTL=15m
DEMO_OPERATOR_PASSWORD=operator-$(openssl rand -hex 4)
DEMO_ADMIN_PASSWORD=admin-$(openssl rand -hex 4)
DEMO_SEED_COUNT=200
EOF
fi

echo "==> Building and starting the demo stack"
docker compose -f "$demo_dir/docker-compose.yml" --env-file "$env_file" up -d --build

# shellcheck source=/dev/null
source "$env_file"
cat <<EOF

FinWatch demo is starting at https://$DEMO_DOMAIN
(the TLS certificate can take a minute on first start)

Demo logins to share with visitors (synthetic data only):
  operator@example.com / $DEMO_OPERATOR_PASSWORD
  admin@example.com    / $DEMO_ADMIN_PASSWORD
EOF
