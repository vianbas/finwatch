# Public demo deployment

This guide runs a public, read-and-play FinWatch demo on one small Linux host
so other people can log in, browse synthetic transactions and alerts, and
learn how the workflow fits together.

FinWatch is a reference implementation using **synthetic data only**. It is
not a fraud, AML, sanctions or compliance product, and the demo must say so.

## Shape

One host runs the stack in `deploy/demo/docker-compose.yml`:

| Service      | Role                                                        |
| ------------ | ----------------------------------------------------------- |
| `caddy`      | Public entrypoint on 80/443; automatic TLS; single origin.  |
| `web`        | React SPA (nginx), served at `/`.                           |
| `api`        | Go API, served at `/api/*` (prefix stripped), WS `/api/ws`. |
| `db`         | PostgreSQL, not published outside the host.                 |
| `migrate`    | One-shot: applies `apps/api/migrations`.                    |
| `seed-users` | One-shot: creates `operator@example.com` / `admin@example.com`. |
| `seed`       | One-shot: ingests `DEMO_SEED_COUNT` synthetic transactions. |

Serving the SPA and API from one origin keeps CORS trivial and lets the
WebSocket use the same TLS certificate.

## Requirements

- A Linux host with Docker Engine + Compose v2 (1 vCPU / 1 GB RAM is enough).
- A DNS name whose A/AAAA record points at the host, with ports 80 and 443
  open (Caddy needs both to obtain the certificate).

## Free hosting: Oracle Cloud Always Free

An Always Free Ampere (arm64) VM runs the whole stack unchanged; every image
used here is published for arm64. Free-tier terms change, so check the
provider's current limits before you start.

1. **Create the VM.** In the Oracle Cloud console, create a Compute instance
   with an Ubuntu 22.04 or 24.04 image and the `VM.Standard.A1.Flex` shape
   (1 OCPU / 6 GB is plenty). Add your SSH public key. If the region reports
   no capacity, retry later or pick another availability domain.
2. **Open ports 80 and 443 in the cloud firewall.** In the instance's VCN →
   subnet → security list, add ingress rules for TCP 80 and TCP 443 (and UDP
   443 for HTTP/3) from `0.0.0.0/0`. The host firewall is handled by the
   setup script.
3. **Pick a hostname.** No domain purchase is needed: `sslip.io` resolves a
   hostname that embeds an IP to that IP, so a VM at `203.0.113.10` can use
   `203-0-113-10.sslip.io`, and Caddy obtains a real certificate for it.
4. **Run the setup script** over SSH:

   ```sh
   sudo apt-get update && sudo apt-get install -y git
   git clone https://github.com/vianbas/finwatch.git && cd finwatch
   sudo deploy/demo/setup-host.sh 203-0-113-10.sslip.io
   ```

   It installs Docker, opens 80/443 in the host's iptables (Oracle's Ubuntu
   images reject them by default), writes `deploy/demo/demo.env` with fresh
   random secrets, starts the stack, and prints the demo logins to share.
   Re-running it keeps the existing `demo.env`.

The commands under *Operating the demo* need `sudo` on this host.

## Deploy on any other host

```sh
git clone https://github.com/vianbas/finwatch.git && cd finwatch
cp deploy/demo/demo.env.example deploy/demo/demo.env
# Fill in DEMO_DOMAIN and generate fresh values:
#   POSTGRES_PASSWORD   openssl rand -hex 24
#   JWT_SIGNING_SECRET  openssl rand -hex 32
#   DEMO_*_PASSWORD     throwaway values you will publish to visitors
docker compose -f deploy/demo/docker-compose.yml --env-file deploy/demo/demo.env up -d --build
```

Check it:

```sh
curl https://$DEMO_DOMAIN/api/health/ready   # {"status":"ready"}
```

## Operating the demo

Simulate more traffic (ingests another batch and raises alerts):

```sh
docker compose -f deploy/demo/docker-compose.yml --env-file deploy/demo/demo.env run --rm seed
```

Reset to a clean, freshly seeded state (visitors can acknowledge and resolve
alerts, so a periodic reset keeps the demo useful). A nightly cron entry works:

```sh
docker compose -f deploy/demo/docker-compose.yml --env-file deploy/demo/demo.env down -v
docker compose -f deploy/demo/docker-compose.yml --env-file deploy/demo/demo.env up -d
```

## Security notes

- `APP_ENV=production` makes the API refuse the published example JWT secret
  and the development fallback demo passwords, so every value in `demo.env`
  must be set explicitly. `demo.env` is git-ignored.
- The demo passwords are intentionally public. Anyone can log in, including as
  admin, so never reuse them elsewhere and never load non-synthetic data.
- Postgres has no published port; only containers on the compose network reach
  it.
- Caddy adds HSTS, `nosniff`, `no-referrer` and `X-Frame-Options: DENY`.
