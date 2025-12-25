# Setup Wildcard SSL dengan Cloudflare

Panduan lengkap setup wildcard SSL certificate (*.grok.cloud) menggunakan Let's Encrypt + Cloudflare DNS.

## Kenapa Perlu DNS-01 Challenge?

- Wildcard certificate (*.grok.cloud) **TIDAK BISA** pakai HTTP-01 challenge
- Let's Encrypt memerlukan **DNS-01 challenge** untuk wildcard
- Cloudflare API akan otomatis handle DNS challenge

## Prerequisites

1. Domain sudah di Cloudflare
2. Cloudflare API Token
3. Server Linux dengan certbot

## Step 1: Setup Cloudflare API Token

### 1.1 Buat API Token di Cloudflare

1. Login ke [Cloudflare Dashboard](https://dash.cloudflare.com)
2. Klik profile (kanan atas) → **My Profile**
3. Klik **API Tokens** → **Create Token**
4. Pilih template: **Edit zone DNS**
5. Configure:
   - **Permissions**: Zone - DNS - Edit
   - **Zone Resources**: Include - Specific zone - grok.cloud
6. **Continue to summary** → **Create Token**
7. **Copy token** (hanya muncul sekali!)

### 1.2 Test API Token

```bash
# Test token (ganti YOUR_TOKEN)
curl -X GET "https://api.cloudflare.com/client/v4/user/tokens/verify" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type:application/json"

# Response harus:
# {"result":{"id":"...","status":"active"},"success":true}
```

## Step 2: Install Certbot + Cloudflare Plugin

### Ubuntu/Debian

```bash
# Install certbot dan cloudflare plugin
sudo apt update
sudo apt install certbot python3-certbot-dns-cloudflare -y
```

### CentOS/RHEL

```bash
sudo yum install certbot python3-certbot-dns-cloudflare -y
```

### macOS (untuk testing)

```bash
brew install certbot
pip3 install certbot-dns-cloudflare
```

## Step 3: Setup Cloudflare Credentials

```bash
# Buat directory untuk credentials
sudo mkdir -p /root/.secrets
sudo chmod 700 /root/.secrets

# Buat credential file
sudo tee /root/.secrets/cloudflare.ini > /dev/null << EOF
# Cloudflare API token
dns_cloudflare_api_token = YOUR_CLOUDFLARE_API_TOKEN_HERE
EOF

# Set permission (PENTING!)
sudo chmod 600 /root/.secrets/cloudflare.ini
```

**PENTING:** Ganti `YOUR_CLOUDFLARE_API_TOKEN_HERE` dengan token dari Step 1!

## Step 4: Request Wildcard Certificate

```bash
# Request wildcard + base domain certificate
sudo certbot certonly \
  --dns-cloudflare \
  --dns-cloudflare-credentials /root/.secrets/cloudflare.ini \
  --dns-cloudflare-propagation-seconds 20 \
  --email admin@grok.cloud \
  --agree-tos \
  --no-eff-email \
  -d grok.cloud \
  -d "*.grok.cloud"
```

### Output yang Diharapkan:

```
Saving debug log to /var/log/letsencrypt/letsencrypt.log
Requesting a certificate for grok.cloud and *.grok.cloud
Waiting 20 seconds for DNS changes to propagate

Successfully received certificate.
Certificate is saved at: /etc/letsencrypt/live/grok.cloud/fullchain.pem
Key is saved at:         /etc/letsencrypt/live/grok.cloud/privkey.pem
This certificate expires on 2025-03-25.
```

## Step 5: Verify Certificate

```bash
# Check certificate files
sudo ls -la /etc/letsencrypt/live/grok.cloud/

# Output:
# fullchain.pem  -> ../../archive/grok.cloud/fullchain1.pem
# privkey.pem    -> ../../archive/grok.cloud/privkey1.pem
# cert.pem       -> ../../archive/grok.cloud/cert1.pem
# chain.pem      -> ../../archive/grok.cloud/chain1.pem

# View certificate details
sudo openssl x509 -in /etc/letsencrypt/live/grok.cloud/fullchain.pem -text -noout | grep -A2 "Subject:"

# Check wildcard
sudo openssl x509 -in /etc/letsencrypt/live/grok.cloud/fullchain.pem -text -noout | grep "DNS:"
# Output harus ada: DNS:grok.cloud, DNS:*.grok.cloud
```

## Step 6: Configure Grok Server

### 6.1 Copy Certificate ke Grok Directory (Opsional)

```bash
# Buat cert directory
sudo mkdir -p /opt/grok/certs

# Copy certificates
sudo cp /etc/letsencrypt/live/grok.cloud/fullchain.pem /opt/grok/certs/
sudo cp /etc/letsencrypt/live/grok.cloud/privkey.pem /opt/grok/certs/

# Set ownership
sudo chown -R grok:grok /opt/grok/certs
sudo chmod 600 /opt/grok/certs/privkey.pem
```

### 6.2 Update Grok Config

**configs/server.yaml:**

```yaml
server:
  grpc_port: 4443
  http_port: 80       # Untuk redirect HTTP → HTTPS
  https_port: 443     # HTTPS dengan wildcard cert
  api_port: 4040
  domain: "grok.cloud"

database:
  driver: "postgres"
  # ... database config ...

tls:
  auto_cert: false    # Disable autocert
  cert_file: "/opt/grok/certs/fullchain.pem"
  key_file: "/opt/grok/certs/privkey.pem"

  # Atau langsung dari letsencrypt:
  # cert_file: "/etc/letsencrypt/live/grok.cloud/fullchain.pem"
  # key_file: "/etc/letsencrypt/live/grok.cloud/privkey.pem"
```

### 6.3 Test Grok Server

```bash
# Start server
sudo systemctl start grok-server

# Check logs
sudo journalctl -u grok-server -f

# Harus muncul:
# TLS enabled auto_cert=false domain=grok.cloud
# HTTPS proxy server listening addr=:443
```

## Step 7: Setup Auto-Renewal

Certbot otomatis install systemd timer untuk renewal.

### 7.1 Verify Renewal Timer

```bash
# Check timer status
sudo systemctl status certbot.timer

# Should be: active (waiting)
```

### 7.2 Test Renewal (Dry Run)

```bash
# Test renewal tanpa actually renew
sudo certbot renew --dry-run

# Output harus:
# Congratulations, all simulated renewals succeeded
```

### 7.3 Setup Reload Hook

Saat certificate di-renew, Grok server perlu reload certificate:

```bash
# Buat reload hook
sudo tee /etc/letsencrypt/renewal-hooks/deploy/reload-grok.sh > /dev/null << 'EOF'
#!/bin/bash
# Reload Grok server after certificate renewal

# Copy new certs
cp /etc/letsencrypt/live/grok.cloud/fullchain.pem /opt/grok/certs/
cp /etc/letsencrypt/live/grok.cloud/privkey.pem /opt/grok/certs/
chown -R grok:grok /opt/grok/certs
chmod 600 /opt/grok/certs/privkey.pem

# Reload Grok server
systemctl reload grok-server

echo "Grok server reloaded with new certificates"
EOF

# Set executable
sudo chmod +x /etc/letsencrypt/renewal-hooks/deploy/reload-grok.sh
```

## Step 8: DNS Configuration

Setup DNS records di Cloudflare:

```
Type    Name              Content              Proxy   TTL
A       grok.cloud        YOUR_SERVER_IP       No      Auto
A       *.grok.cloud      YOUR_SERVER_IP       No      Auto
A       tunnel            YOUR_SERVER_IP       No      Auto
```

**PENTING:**
- **Proxy harus OFF** untuk wildcard dan gRPC
- Jika proxy ON, Cloudflare akan terminate TLS dan break gRPC

## Testing

### Test HTTPS Wildcard

```bash
# Test base domain
curl -I https://grok.cloud

# Test wildcard subdomain
curl -I https://tunnel.grok.cloud
curl -I https://abc123.grok.cloud
curl -I https://anything.grok.cloud

# All should return 200 or valid response
```

### Test dengan Client

```bash
# Start client
./bin/grok http 8000 --server grok.cloud:4443

# Harus dapat subdomain:
# Public URL: https://xyz789.grok.cloud
```

## Troubleshooting

### Certificate Request Gagal

```bash
# Check Cloudflare API token
curl -X GET "https://api.cloudflare.com/client/v4/user/tokens/verify" \
  -H "Authorization: Bearer YOUR_TOKEN"

# Check DNS propagation
dig _acme-challenge.grok.cloud TXT

# Increase propagation time
sudo certbot certonly \
  --dns-cloudflare \
  --dns-cloudflare-propagation-seconds 60 \
  ...
```

### Permission Denied

```bash
# Fix letsencrypt permissions
sudo chmod 755 /etc/letsencrypt/live
sudo chmod 755 /etc/letsencrypt/archive

# Or run Grok as root (not recommended)
sudo ./bin/grok-server
```

### Auto-Renewal Gagal

```bash
# Check renewal status
sudo certbot renew --dry-run

# Manual renewal
sudo certbot renew --force-renewal

# Check renewal logs
sudo cat /var/log/letsencrypt/letsencrypt.log
```

## Certificate Info

```bash
# Expiration date
sudo certbot certificates

# Output:
# Certificate Name: grok.cloud
#   Domains: grok.cloud *.grok.cloud
#   Expiry Date: 2025-03-25 (59 days)
#   Certificate Path: /etc/letsencrypt/live/grok.cloud/fullchain.pem
#   Private Key Path: /etc/letsencrypt/live/grok.cloud/privkey.pem
```

## Backup Certificates

```bash
# Backup semua certificates
sudo tar -czf letsencrypt-backup-$(date +%Y%m%d).tar.gz /etc/letsencrypt/

# Restore
sudo tar -xzf letsencrypt-backup-20250125.tar.gz -C /
```

## Kesimpulan

Setelah setup selesai:

✅ Wildcard certificate (*.grok.cloud) aktif
✅ HTTPS otomatis untuk semua subdomain
✅ Auto-renewal setiap 60 hari
✅ Grok server auto-reload certificate

**Maintenance:** Tidak perlu! Certbot akan auto-renew dan reload server.
