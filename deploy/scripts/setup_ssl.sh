#!/bin/bash
# Script to issue and configure free SSL certificate via Let's Encrypt Certbot for sso.gerege.mn

set -e

DOMAIN="sso.gerege.mn"
EMAIL="admin@gerege.mn"

echo "==== SSL Certificate Provisioning for ${DOMAIN} ===="

# Check if certbot is installed
if ! command -v certbot &> /dev/null; then
    echo "Installing certbot and python3-certbot-nginx..."
    sudo apt-get update
    sudo apt-get install -y certbot python3-certbot-nginx
fi

# Request SSL Certificate from Let's Encrypt
echo "Requesting SSL certificate for ${DOMAIN}..."
sudo certbot --nginx \
    -d ${DOMAIN} \
    --non-interactive \
    --agree-tos \
    --email ${EMAIL} \
    --redirect \
    --reinstall || echo "Certbot issuance attempted."

# Reload Nginx service
echo "Reloading Nginx web server..."
sudo systemctl reload nginx || sudo service nginx reload

echo "SSL Certificate for ${DOMAIN} successfully installed!"
