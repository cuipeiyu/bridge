#!/usr/bin/env bash

set -e

clear

echo 'Download'
curl -o /tmp/bridge/bridge-linux-amd64.tar.gz https://f.cuipeiyu.com/bridge/bridge-linux-amd64.tar.gz

echo 'Install'
tar -vxf /tmp/bridge/bridge-linux-amd64.tar.gz -C /tmp/bridge

cp -f /tmp/bridge/bridge /usr/local/sbin
cp -f /tmp/bridge/bridge.conf /usr/local/sbin

chmod +x /usr/local/sbin/bridge

cp -f /tmp/bridge/bridge.service /usr/lib/systemd/system

echo 'Clean'

rm -rf /tmp/bridge

systemctl enable bridge

echo 'Done'
