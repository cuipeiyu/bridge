#!/usr/bin/env bash

set -e

clear

echo 'Download...'
mkdir -p /tmp/bridge
curl -fsSL -o /tmp/bridge/bridge-linux-amd64.tar.gz https://github.com/cuipeiyu/bridge/releases/latest/download/bridge-linux-amd64.tar.gz

echo 'Installing...'
tar -vxf /tmp/bridge/bridge-linux-amd64.tar.gz -C /tmp/bridge

cp -f /tmp/bridge/bridge /usr/local/sbin
cp -f /tmp/bridge/bridge.conf /usr/local/sbin

chmod +x /usr/local/sbin/bridge

cp -f /tmp/bridge/bridge.service /usr/lib/systemd/system

systemctl enable bridge

rm -rf /tmp/bridge

echo 'Done'
