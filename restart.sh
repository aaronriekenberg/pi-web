#!/bin/sh -x

KILL_CMD=pkill
CONFIG_FILE=config/$(hostname -s)-config.json

$KILL_CMD pi-web

sleep 2

nohup ./pi-web $CONFIG_FILE >> logs/output 2>&1 &
