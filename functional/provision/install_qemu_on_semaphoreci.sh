#!/bin/sh

echo "Installing QEMU..."
sudo apt-get update -qq
sudo apt-get install -qq -y qemu-system-x86
# Fix "initctl: Unknown job: qemu-kvm" bug
sudo initctl --system start qemu-kvm || true
