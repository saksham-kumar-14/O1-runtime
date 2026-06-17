#!/bin/bash

set -e

if [ "$EUID" -ne 0 ]; then
    echo "❌ Error: Run as root (sudo ./setup.sh)"
    exit 1
fi

if ! command -v go &> /dev/null; then
    echo "❌ Error: Go is not installed on this system."
    exit 1
fi

echo "Compiling the o1 engine..."


CGO_ENABLED=1 go build -o /tmp/o1 ./cmd/run/main.go
mv /tmp/o1 /usr/local/bin/o1
chmod +x /usr/local/bin/o1

mkdir -p /var/lib/o1/images
mkdir -p /var/lib/o1/containers
mkdir -p /var/lib/o1/state
chmod -R 755 /var/lib/o1

echo ""
echo "o1 engine successfully installed!"
echo "Try it out: sudo o1 pull alpine && sudo o1 run alpine /bin/sh"
