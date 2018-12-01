#!/bin/sh -x

KILL_CMD=pkill
CONFIG_FILE=config/$(hostname)-config.yml

$KILL_CMD pi-web

sleep 2

nohup ./pi-web $CONFIG_FILE 2>&1 | svlogd logs &
