#!/bin/sh -x

KILL_CMD=killall
CONFIG_FILE=config.yml

$KILL_CMD pi-web

rm -f pi-web.out

nohup ./pi-web $CONFIG_FILE > pi-web.out 2>&1 &
