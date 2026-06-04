#!/bin/bash

set -e
if [ "$EUID" -ne 0 ]; then
    echo "Run the fs script, eg: sudo ./fs.sh alpine"
    exit 1
fi

DISTRO=$1
TARGET_DIR="/var/lib/o1/images/default"

if [ -z "$DISTRO" ]; then
  echo "Usage: sudo ./fs.sh [alpine|arch]"
  exit 1
fi

echo "Target Directory: $TARGET_DIR"
echo "Architecture: Forced to aarch64"

if [ "$DISTRO" = "alpine" ]; then
  URL="https://dl-cdn.alpinelinux.org/alpine/v3.19/releases/aarch64/alpine-minirootfs-3.19.1-aarch64.tar.gz"

  echo "Downloading, extracting and cleaning alpine.."
  rm -rf $TARGET_DIR
  mkdir -p $TARGET_DIR
  wget -qO /tmp/alpine.tar.gz $URL
  tar -xzf /tmp/alpine.tar.gz -C $TARGET_DIR
  rm /tmp/alpine.tar.gz

  echo "Alpine filesystem is ready at $TARGET_DIR!"

elif [ "$DISTRO" = "arch" ]; then
  URL="http://os.archlinuxarm.org/os/ArchLinuxARM-aarch64-latest.tar.gz"

  rm -rf $TARGET_DIR

  echo "Downloading, extracting and cleaning arch.."
  mkdir -p $TARGET_DIR
  wget -qO /tmp/arch.tar.gz $URL
  tar -xzf /tmp/arch.tar.gz -C $TARGET_DIR
  rm /tmp/arch.tar.gz

  echo "Arch filesystem is ready at $TARGET_DIR!"

else
  echo "Error: Unknown distribution '$DISTRO'. Please use 'alpine' or 'arch'."
  exit 1
fi
