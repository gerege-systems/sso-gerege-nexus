#!/usr/bin/env bash
# Bring a freshly installed Ubuntu host to the state deploy.yml assumes.
#
# deploy.yml does not provision anything — it scp's docker-compose.prod.yml to
# $APP_DIR, writes .env, and runs `docker compose up -d`. Everything that has to
# exist before that first rollout is here.
#
# Run as root on the new box, from a checkout of this repository — the nginx
# site file is read from ../nginx relative to this script:
#     DEPLOY_PUBKEY="ssh-ed25519 AAAA... gerege-deploy" bash deploy/scripts/provision-fresh-server.sh
#
# Idempotent: safe to re-run. In particular a re-run will not undo certbot's
# work — once the site file carries a TLS block it is left alone.

set -euo pipefail

DEPLOY_USER="${DEPLOY_USER:-deploy}"
# deploy.yml hardcodes this path (APP_DIR=/opt/sso-gerege-mn-erp in the rollout
# script and as the scp target), so overriding it here only makes sense
# alongside an edit there.
#
# It keeps the pre-rename spelling on purpose, though the repository is now
# sso-gerege-nexus. Compose derives its project name from this directory and
# scopes the Postgres volume with it, so renaming the directory points the stack
# at a volume that does not exist: initdb runs, migrations apply, and the result
# is a healthy container over an empty database. Moving it is a data migration —
# stop the stack, copy the volume, repoint both this and deploy.yml — not a
# tidy-up.
APP_DIR="${APP_DIR:-/opt/sso-gerege-mn-erp}"
DOMAIN="${DOMAIN:-sso.gerege.mn}"
CERTBOT_EMAIL="${CERTBOT_EMAIL:-admin@gerege.mn}"
# Must match the private half stored in the DEPLOY_SSH_KEY repository secret.
DEPLOY_PUBKEY="${DEPLOY_PUBKEY:-}"
# The firewall is enabled below while you are connected over SSH. Opening only
# port 22 would lock out both you and a rollout using a non-default DEPLOY_PORT,
# so take the port from the running sshd unless told otherwise.
SSH_PORT="${SSH_PORT:-}"
if [ -z "$SSH_PORT" ]; then
  SSH_PORT="$(sshd -T 2>/dev/null | awk '/^port /{print $2; exit}')" || true
  SSH_PORT="${SSH_PORT:-22}"
fi

if [ -z "$DEPLOY_PUBKEY" ]; then
  echo "DEPLOY_PUBKEY is required — the rollout authenticates as $DEPLOY_USER with it." >&2
  exit 1
fi

if [ "$APP_DIR" != "/opt/sso-gerege-mn-erp" ]; then
  echo "warning: APP_DIR is $APP_DIR but deploy.yml deploys to /opt/sso-gerege-mn-erp." >&2
fi

echo "==> packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq ca-certificates curl gnupg nginx certbot python3-certbot-nginx ufw

echo "==> docker engine + compose plugin"
# Ubuntu's docker.io package ships no `docker compose` subcommand, and the
# rollout calls `docker compose`, not `docker-compose`. So the test is for the
# subcommand, not for a `docker` binary — a host carrying docker.io has the
# binary and still cannot run the rollout.
if ! docker compose version >/dev/null 2>&1; then
  install -m 0755 -d /etc/apt/keyrings
  # --yes: a re-run after a half-finished attempt would otherwise die on
  # "File exists".
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg \
    | gpg --yes --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update -qq
  if command -v docker >/dev/null 2>&1; then
    # An engine is already running (docker.io, most likely). Pulling in
    # docker-ce here would make apt remove it mid-provision; all that is
    # missing is the compose plugin.
    apt-get install -y -qq docker-compose-plugin
  else
    apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  fi
fi
systemctl enable --now docker
docker compose version

echo "==> deploy user"
id -u "$DEPLOY_USER" >/dev/null 2>&1 || adduser --disabled-password --gecos "" "$DEPLOY_USER"
usermod -aG docker "$DEPLOY_USER"
install -d -m 0700 -o "$DEPLOY_USER" -g "$DEPLOY_USER" "/home/$DEPLOY_USER/.ssh"
touch "/home/$DEPLOY_USER/.ssh/authorized_keys"
grep -qxF "$DEPLOY_PUBKEY" "/home/$DEPLOY_USER/.ssh/authorized_keys" \
  || echo "$DEPLOY_PUBKEY" >> "/home/$DEPLOY_USER/.ssh/authorized_keys"
chmod 600 "/home/$DEPLOY_USER/.ssh/authorized_keys"
chown -R "$DEPLOY_USER:$DEPLOY_USER" "/home/$DEPLOY_USER/.ssh"

echo "==> app dir"
# scp-action writes docker-compose.prod.yml here and the rollout writes .env
# next to it, both as $DEPLOY_USER.
install -d -m 0755 -o "$DEPLOY_USER" -g "$DEPLOY_USER" "$APP_DIR"

echo "==> nginx site"
SITE_SRC="$(dirname "$0")/../nginx/${DOMAIN}.conf"
SITE_DEST="/etc/nginx/sites-available/${DOMAIN}"
if grep -qs "ssl_certificate" "$SITE_DEST"; then
  # Certbot has already rewritten this file to add the 443 block. Copying the
  # HTTP-only source over it would silently drop the site back to plain HTTP —
  # and the "certificate already present" branch below would then skip
  # reissuing it. Merge any source changes by hand instead.
  echo "  ${SITE_DEST} already carries certbot's TLS block — left untouched."
  echo "  Apply changes from ${SITE_SRC} by hand if the source has moved on."
elif [ -f "$SITE_SRC" ]; then
  cp "$SITE_SRC" "$SITE_DEST"
else
  # Do not fall through: the symlink below would then point at nothing, and
  # `nginx -t` would fail for every site on this host, not just this one.
  echo "${SITE_SRC} not found. Run this script from a checkout of the repository," >&2
  echo "or copy the site config to ${SITE_DEST} first." >&2
  exit 1
fi
ln -sfn "$SITE_DEST" "/etc/nginx/sites-enabled/${DOMAIN}"
# Only the stock Debian placeholder. This host also serves nexus.gerege.mn,
# and removing a `default` symlink that points at a real site would take that
# deployment offline.
if [ "$(readlink -f /etc/nginx/sites-enabled/default 2>/dev/null || true)" = "/etc/nginx/sites-available/default" ]; then
  rm -f /etc/nginx/sites-enabled/default
fi
nginx -t
systemctl reload nginx

echo "==> firewall"
ufw allow "${SSH_PORT}/tcp"
ufw allow 'Nginx Full'
ufw --force enable

echo "==> tls"
# Requires the DNS A record for $DOMAIN to already point at this host, and
# certbot rewrites the site file to add the 443 block.
if [ -d "/etc/letsencrypt/live/${DOMAIN}" ] && grep -qs "ssl_certificate" "$SITE_DEST"; then
  echo "  certificate and TLS block already in place, skipping issuance"
else
  certbot_args=(--nginx -d "$DOMAIN" --non-interactive --agree-tos
    --email "$CERTBOT_EMAIL" --redirect)
  if [ -d "/etc/letsencrypt/live/${DOMAIN}" ]; then
    # The certificate exists but the site file lost its 443 block. Reinstall it
    # rather than leaving the site reachable over plain HTTP only.
    echo "  certificate present but the site has no TLS block — reinstalling"
    certbot_args+=(--reinstall)
  fi
  if ! certbot "${certbot_args[@]}"; then
    echo "::error:: certbot failed — check that ${DOMAIN} resolves to this host, then re-run." >&2
    exit 1
  fi
  nginx -t
  systemctl reload nginx
fi

cat <<EOF

==> done. Remaining steps live outside this box:

  1. Point the DNS A record for ${DOMAIN} at this host, if not already.
  2. Update the DEPLOY_HOST repository secret to this host's IP:
       gh secret set DEPLOY_HOST -R gerege-systems/sso-gerege-nexus
  3. Confirm DEPLOY_USER matches "${DEPLOY_USER}", DEPLOY_PORT matches
     ${SSH_PORT}, and DEPLOY_SSH_KEY holds the private half of the key
     installed above.
  4. Run the deploy workflow:
       gh workflow run deploy.yml -R gerege-systems/sso-gerege-nexus --ref main

EOF
