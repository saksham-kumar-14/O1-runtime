#!/bin/bash

set -e

if [ "$EUID" -ne 0 ]; then
    echo "❌ Error: run as sudo ./setup.sh"
    exit 1
fi

if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    exit 1
fi

GOOS=linux go build -o /tmp/o1 ./cmd/run/main.go
mv /tmp/o1 /usr/local/bin/o1
chmod +x /usr/local/bin/o1


mkdir -p /var/lib/o1/images/default
mkdir -p /var/lib/o1/containers
mkdir -p /var/lib/o1/state
chmod -R 755 /var/lib/o1


if [ -f "./fs/fs.sh" ]; then
    echo "Installing 'fs.sh' as 'o1-fs'..."
    cp ./fs/fs.sh /usr/local/bin/o1-fs
    chmod +x /usr/local/bin/o1-fs
else
    echo "'./fs/fs.sh' not found"
fi

echo ""
echo "O1 engine successfully installed!"
