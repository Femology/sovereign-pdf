#!/usr/bin/env bash
# install_deps.sh — Install all system dependencies for Sovereign PDF
# Usage: sudo ./install_deps.sh
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root: sudo $0"
  exit 1
fi

echo "==> Updating package lists..."
apt-get update -y

echo "==> Installing Sovereign PDF engine dependencies..."
DEBIAN_FRONTEND=noninteractive apt-get install -y \
  ghostscript \
  poppler-utils \
  tesseract-ocr \
  libreoffice-writer \
  libreoffice-calc \
  libreoffice-impress

echo ""
echo "==> Verifying installations..."
for bin in gs pdftotext pdftoppm tesseract soffice; do
  if command -v "$bin" >/dev/null 2>&1; then
    echo "  ✓ $bin — $(command -v "$bin")"
  else
    echo "  ✗ $bin — NOT FOUND"
  fi
done

echo ""
echo "All dependencies installed. Restart Sovereign PDF:"
echo "  ./sovereign-pdf"
