# Docker Image Attestation & Verification

This document explains how to verify the authenticity and integrity of Grok Docker images using artifact attestation and image signing.

## What is Artifact Attestation?

Artifact attestation provides cryptographic proof about:
- **Build Provenance**: Who built the image, when, where, and from what source code
- **SBOM**: Complete list of all software dependencies
- **Integrity**: Guarantee that the image hasn't been tampered with

## Security Features

Our Docker images include multiple layers of security:

### 1. **Build Provenance Attestation**
- Proves the image was built by GitHub Actions
- Contains commit SHA, workflow details, and build environment
- Signed with GitHub's OIDC token

### 2. **SBOM (Software Bill of Materials)**
- Complete dependency list in SPDX format
- Enables vulnerability tracking and license compliance
- Automatically generated during build

### 3. **Cosign Signatures** (optional)
- Keyless signing using Sigstore
- Verifiable without managing private keys
- Integration with transparency logs

### 4. **Security Scanning**
- Trivy vulnerability scanner
- Results published to GitHub Security tab
- Blocks deployment of critical vulnerabilities

---

## Verifying Docker Images

### Prerequisites

Install required tools:
```bash
# GitHub CLI (for attestation verification)
brew install gh  # macOS
# or
sudo apt install gh  # Ubuntu/Debian

# Cosign (for signature verification)
brew install cosign  # macOS
# or
curl -LO https://github.com/sigstore/cosign/releases/latest/download/cosign-linux-amd64
sudo mv cosign-linux-amd64 /usr/local/bin/cosign
sudo chmod +x /usr/local/bin/cosign
```

---

### Method 1: Verify Build Provenance (Recommended)

Verify the image was built from the official GitHub repository:

```bash
# Get image digest
IMAGE="ghcr.io/pandeptwidyaop/grok"
DIGEST=$(docker inspect --format='{{.RepoDigests}}' $IMAGE:latest | grep -oP 'sha256:[a-f0-9]+')

# Verify attestation
gh attestation verify oci://$IMAGE@$DIGEST \
  --owner pandeptwidyaop
```

**Expected Output:**
```
✓ Verification succeeded!

Attestation verified for ghcr.io/pandeptwidyaop/grok@sha256:...

Issuer: https://token.actions.githubusercontent.com
Workflow: .github/workflows/docker-release.yml
Repository: pandeptwidyaop/grok
Commit: abc1234...
```

---

### Method 2: Verify Cosign Signature

Verify the cryptographic signature:

```bash
IMAGE="ghcr.io/pandeptwidyaop/grok"
DIGEST=$(docker inspect --format='{{.RepoDigests}}' $IMAGE:latest | grep -oP 'sha256:[a-f0-9]+')

cosign verify $IMAGE@$DIGEST \
  --certificate-identity=https://github.com/pandeptwidyaop/grok/.github/workflows/docker-release.yml@refs/heads/main \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

**Expected Output:**
```
Verification for ghcr.io/pandeptwidyaop/grok@sha256:... --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - Existence of the claims in the transparency log was verified offline
  - The code-signing certificate was verified using trusted certificate authority certificates

[{"critical":{"identity":{"docker-reference":"ghcr.io/pandeptwidyaop/grok"},...}]
```

---

### Method 3: Inspect SBOM

View the Software Bill of Materials:

```bash
IMAGE="ghcr.io/pandeptwidyaop/grok"
DIGEST=$(docker inspect --format='{{.RepoDigests}}' $IMAGE:latest | grep -oP 'sha256:[a-f0-9]+')

# Download and view SBOM
gh attestation verify oci://$IMAGE@$DIGEST \
  --owner pandeptwidyaop \
  --format json | jq '.subject.sbom'
```

This shows all dependencies including:
- Go modules and versions
- Operating system packages
- npm packages (from web dashboard)
- Exact versions for vulnerability tracking

---

### Method 4: Manual Provenance Inspection

Download and inspect the full provenance:

```bash
IMAGE="ghcr.io/pandeptwidyaop/grok:latest"

# Download provenance
gh attestation download oci://$IMAGE --owner pandeptwidyaop

# Inspect the attestation
cat attestation.json | jq '.predicate'
```

**Provenance contains:**
- Build timestamp
- Git commit SHA
- GitHub Actions workflow details
- Build environment (OS, architecture)
- Materials (source files) used in build
- Build steps and parameters

---

## Production Deployment Best Practices

### 1. Always Use Digests

❌ **Don't:**
```yaml
docker:
  image: ghcr.io/pandeptwidyaop/grok:latest
```

✅ **Do:**
```yaml
docker:
  image: ghcr.io/pandeptwidyaop/grok@sha256:abc123...
```

**Why:** Tags are mutable, digests are immutable. Ensures you're running exactly the verified image.

### 2. Verify Before Deployment

```bash
#!/bin/bash
# pre-deploy.sh

IMAGE="ghcr.io/pandeptwidyaop/grok"
TAG="v1.2.3"

# Pull image
docker pull $IMAGE:$TAG

# Get digest
DIGEST=$(docker inspect --format='{{.RepoDigests}}' $IMAGE:$TAG | grep -oP 'sha256:[a-f0-9]+')

# Verify attestation
if gh attestation verify oci://$IMAGE@$DIGEST --owner pandeptwidyaop; then
  echo "✅ Image verified successfully"
  echo "Deploying $IMAGE@$DIGEST"
  # Deploy...
else
  echo "❌ Image verification failed!"
  exit 1
fi
```

### 3. Enforce Verification in CI/CD

**GitHub Actions:**
```yaml
- name: Verify Docker image
  run: |
    gh attestation verify oci://ghcr.io/pandeptwidyaop/grok:${{ github.sha }} \
      --owner pandeptwidyaop
```

**Kubernetes Admission Controller:**
Use Sigstore Policy Controller to enforce signed images:
```yaml
apiVersion: policy.sigstore.dev/v1beta1
kind: ClusterImagePolicy
metadata:
  name: grok-image-policy
spec:
  images:
  - glob: "ghcr.io/pandeptwidyaop/grok**"
  authorities:
  - keyless:
      identities:
      - issuer: https://token.actions.githubusercontent.com
        subject: https://github.com/pandeptwidyaop/grok/.github/workflows/docker-release.yml@refs/heads/main
```

---

## Understanding Attestation Formats

### Build Provenance (SLSA v1.0)

```json
{
  "predicateType": "https://slsa.dev/provenance/v1",
  "subject": [
    {
      "name": "ghcr.io/pandeptwidyaop/grok",
      "digest": {"sha256": "..."}
    }
  ],
  "predicate": {
    "buildDefinition": {
      "buildType": "https://slsa-framework.github.io/github-actions-buildtypes/workflow/v1",
      "externalParameters": {
        "workflow": {
          "ref": "refs/heads/main",
          "repository": "https://github.com/pandeptwidyaop/grok",
          "path": ".github/workflows/docker-release.yml"
        }
      }
    },
    "runDetails": {
      "builder": {
        "id": "https://github.com/actions/runner"
      },
      "metadata": {
        "invocationId": "https://github.com/pandeptwidyaop/grok/actions/runs/..."
      }
    }
  }
}
```

### SBOM (SPDX 2.3)

```json
{
  "spdxVersion": "SPDX-2.3",
  "dataLicense": "CC0-1.0",
  "name": "ghcr.io/pandeptwidyaop/grok",
  "packages": [
    {
      "name": "golang.org/x/crypto",
      "versionInfo": "v0.31.0",
      "downloadLocation": "https://proxy.golang.org/golang.org/x/crypto/@v/v0.31.0.zip",
      "licenseConcluded": "BSD-3-Clause"
    },
    ...
  ]
}
```

---

## Troubleshooting

### Error: "no attestations found"

**Cause:** Image was built before attestation was enabled, or you're using wrong digest.

**Solution:**
```bash
# Make sure you're using the exact digest
docker inspect ghcr.io/pandeptwidyaop/grok:latest | grep RepoDigests

# Verify with correct digest
gh attestation verify oci://ghcr.io/pandeptwidyaop/grok@sha256:EXACT_DIGEST --owner pandeptwidyaop
```

### Error: "verification failed"

**Cause:** Image has been modified, or using wrong owner.

**Solution:**
```bash
# Check image was actually built by GitHub Actions
docker inspect ghcr.io/pandeptwidyaop/grok:latest | jq '.[].Config.Labels'

# Should see:
# "org.opencontainers.image.source": "https://github.com/pandeptwidyaop/grok"
```

### Error: "cosign verify failed"

**Cause:** Certificate identity mismatch.

**Solution:**
```bash
# Get actual certificate details from transparency log
cosign verify ghcr.io/pandeptwidyaop/grok:latest 2>&1 | grep "certificate-identity"

# Use the exact identity shown
```

---

## Security Incident Response

If verification fails:

1. **Do NOT deploy the image**
2. Check GitHub Actions logs for build job
3. Verify the commit SHA matches expected release
4. Contact security team
5. Investigate potential compromise

---

## References

- [SLSA Framework](https://slsa.dev/)
- [GitHub Artifact Attestations](https://docs.github.com/en/actions/security-guides/using-artifact-attestations-to-establish-provenance-for-builds)
- [Sigstore Cosign](https://docs.sigstore.dev/cosign/overview/)
- [SPDX SBOM](https://spdx.dev/)
- [Supply Chain Levels for Software Artifacts](https://slsa.dev/spec/v1.0/)
