#!/bin/sh -x

KILL_CMD=pkill
CONFIG_FILE=$(hostname)-config.yml

$KILL_CMD pi-web

sleep 2

nohup ./pi-web $CONFIG_FILE 2>&1 | svlogd logs &
