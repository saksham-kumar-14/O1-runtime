#!/bin/bash

set -e

if [ "$EUID" -ne 0 ]; then
    echo "Run the fs script, eg: sudo ./fs.sh alpine"
    exit 1
fi

DISTRO=$1
ARCH=$(uname -m)

if [ -z "$DISTRO" ]; then
  echo "Usage: sudo ./fs.sh [alpine|arch]"
  exit 1
fi

echo "CPU Architecture: $ARCH"

if [ "$DISTRO" = "alpine" ]; then
  TARGET_DIR="/tmp/alpine-fs"

  if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
    URL="https://dl-cdn.alpinelinux.org/alpine/v3.19/releases/aarch64/alpine-minirootfs-3.19.1-aarch64.tar.gz"
  else
    URL="https://dl-cdn.alpinelinux.org/alpine/v3.19/releases/x86_64/alpine-minirootfs-3.19.1-x86_64.tar.gz"
  fi

  echo "Downloading, extracting and cleaning alpine.."
  rm -rf $TARGET_DIR
  mkdir -p $TARGET_DIR
  wget -qO /tmp/alpine.tar.gz $URL
  tar -xzf /tmp/alpine.tar.gz -C $TARGET_DIR
  rm /tmp/alpine.tar.gz

  echo "Alpine filesystem is ready at $TARGET_DIR!"

elif [ "$DISTRO" = "arch" ]; then
  TARGET_DIR="/tmp/arch-fs"
  rm -rf $TARGET_DIR

  if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
    URL="http://os.archlinuxarm.org/os/ArchLinuxARM-aarch64-latest.tar.gz"

    echo "Downloading, extracting and cleaning arch.."
    mkdir -p $TARGET_DIR
    wget -qO /tmp/arch.tar.gz $URL
    tar -xzf /tmp/arch.tar.gz -C $TARGET_DIR
    rm /tmp/arch.tar.gz
  else
    URL="https://geo.mirror.pkgbuild.com/iso/latest/archlinux-bootstrap-x86_64.tar.zst"

    echo "Downloading, extracting and cleaning arch.."
    wget -qO /tmp/arch.tar.zst $URL
    tar -xf /tmp/arch.tar.zst -C /tmp
    mv /tmp/root.x86_64 $TARGET_DIR
    rm /tmp/arch.tar.zst
  fi

  echo "Arch filesystem is ready at $TARGET_DIR!"

else
  echo "Error: Unknown distribution '$DISTRO'. Please use 'alpine' or 'arch'."
  exit 1
fi
