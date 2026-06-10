#!/usr/bin/env bash

# Exit immediately if a command exits with a non-zero status
set -e

# Define colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Starting Sovereign PDF Build Pipeline...${NC}"

# 1. Build the React Frontend
echo -e "\n${GREEN}[1/6] Building React frontend UI...${NC}"
cd ui
npm install
npm run build
cd ..

# 2. Prepare the embed target directory
echo -e "\n${GREEN}[2/6] Cleaning and preparing embed target directory...${NC}"
rm -rf internal/api/static
mkdir -p internal/api/static

# 3. Copy frontend assets for Go embedding
echo -e "\n${GREEN}[3/6] Copying compiled assets to internal/api/static/...${NC}"
cp -r ui/dist/* internal/api/static/

# 4. Clean Go module dependencies
echo -e "\n${GREEN}[4/6] Tidying Go modules...${NC}"
go mod tidy

# 5. Compile standard local binary
echo -e "\n${GREEN}[5/6] Compiling local optimized production binary...${NC}"
go build -ldflags="-s -w" -o sovereign-pdf ./cmd/server/main.go

# 6. Cross-compile deployment binaries for servers
echo -e "\n${GREEN}[6/6] Cross-compiling for deployment targets...${NC}"
mkdir -p build

echo -e "  -> Building Linux AMD64..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o build/sovereign-pdf-linux-amd64 ./cmd/server/main.go

echo -e "  -> Building Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o build/sovereign-pdf-linux-arm64 ./cmd/server/main.go

echo -e "\n${BLUE}Build pipeline complete! Production binaries are ready:${NC}"
ls -lh sovereign-pdf build/
