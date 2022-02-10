#!/bin/sh

KILL_CMD=pkill
CONFIG_FILE=configfiles/$(hostname -s)-config.json

$KILL_CMD pi-web

sleep 2

export PATH=${HOME}/bin:$PATH

nohup ./pi-web $CONFIG_FILE 2>&1 | go-simplerotate logs &
