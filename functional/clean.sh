#!/bin/bash
#
# WARNING: This script is destructive and should only be run within the test environment
#

machinectl --no-legend | cut -d ' ' -f1 | sudo xargs -r machinectl terminate
sudo pkill -9 systemd-nspawn
sudo rm -fr /run/systemd/system/*smoke* /tmp/smoke
sudo systemctl daemon-reload
ip link show flt0 &>/dev/null && sudo ip link del flt0
etcdctl rm --recursive /flt_functional
