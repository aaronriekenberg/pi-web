#!/bin/sh -x

KILL_CMD=pkill
CONFIG_FILE=$(hostname)-config.yml

$KILL_CMD pi-web

rm -f pi-web.out

nohup ./pi-web $CONFIG_FILE > pi-web.out 2>&1 &
