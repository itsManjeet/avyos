#!/bin/bash
set -euo pipefail

# Mode: by default, generate all standard keys and download MS keys if missing.
# If --download-ms is passed, explicitly download/renew MS keys.
DOWNLOAD_MS=false
if [ "${1:-}" = "--download-ms" ]; then
    DOWNLOAD_MS=true
fi

SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
SIGN_KEYS_DIR="${REPO_ROOT}/files/sign-keys"

# Ensure output directory exists
mkdir -p "$SIGN_KEYS_DIR"

# Generate standard keys if they don't exist
for key in PK KEK DB VENDOR linux-module-cert; do
    key_path="${SIGN_KEYS_DIR}/${key}"
    if [ ! -f "${key_path}.key" ] || [ ! -f "${key_path}.crt" ]; then
        echo "Generating key and cert for ${key}..."
        openssl req -new -x509 -newkey rsa:2048 \
            -subj "/CN=AVYOS ${key} key/" \
            -keyout "${key_path}.key" \
            -out "${key_path}.crt" \
            -days 3650 -nodes -sha256
    fi
done

# Copy linux-module-cert to modules/
mkdir -p "${SIGN_KEYS_DIR}/modules"
cp "${SIGN_KEYS_DIR}/linux-module-cert.crt" "${SIGN_KEYS_DIR}/modules/linux-module-cert.crt"

# Ensure extra directories and keep files exist
mkdir -p "${SIGN_KEYS_DIR}/extra-db" "${SIGN_KEYS_DIR}/extra-kek"
touch "${SIGN_KEYS_DIR}/extra-db/.keep"
touch "${SIGN_KEYS_DIR}/extra-kek/.keep"

# Download Microsoft keys if requested or if they do not exist
if [ "$DOWNLOAD_MS" = true ] || \
   [ ! -f "${SIGN_KEYS_DIR}/extra-kek/mic-kek.crt" ] || \
   [ ! -f "${SIGN_KEYS_DIR}/extra-db/mic-other.crt" ] || \
   [ ! -f "${SIGN_KEYS_DIR}/extra-db/mic-win.crt" ]; then
    echo "Downloading/Updating Microsoft keys..."
    curl -sSf https://www.microsoft.com/pkiops/certs/MicCorUEFCA2011_2011-06-27.crt | openssl x509 -inform der -outform pem > "${SIGN_KEYS_DIR}/extra-kek/mic-kek.crt"
    echo 77fa9abd-0359-4d32-bd60-28f4e78f784b > "${SIGN_KEYS_DIR}/extra-kek/mic-kek.owner"

    curl -sSf https://www.microsoft.com/pkiops/certs/MicCorUEFCA2011_2011-06-27.crt | openssl x509 -inform der -outform pem > "${SIGN_KEYS_DIR}/extra-db/mic-other.crt"
    echo 77fa9abd-0359-4d32-bd60-28f4e78f784b > "${SIGN_KEYS_DIR}/extra-db/mic-other.owner"

    curl -sSf https://www.microsoft.com/pkiops/certs/MicWinProPCA2011_2011-10-19.crt | openssl x509 -inform der -outform pem > "${SIGN_KEYS_DIR}/extra-db/mic-win.crt"
    echo 77fa9abd-0359-4d32-bd60-28f4e78f784b > "${SIGN_KEYS_DIR}/extra-db/mic-win.owner"
fi
