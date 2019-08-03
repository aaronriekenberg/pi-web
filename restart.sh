#!/bin/sh -x

KILL_CMD=pkill
CONFIG_FILE=config/$(hostname -s)-config.json

$KILL_CMD pi-web

sleep 2

export PATH=${HOME}/bin:$PATH

nohup ./pi-web $CONFIG_FILE 2>&1 | svlogd logs &
