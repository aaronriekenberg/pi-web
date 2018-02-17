#!/bin/sh -x

KILL_CMD=killall
CONFIG_FILE=config.yml

if [ $(uname) = 'OpenBSD' ]; then
  KILL_CMD=pkill
  CONFIG_FILE=openbsd-config.yml
fi

$KILL_CMD pi-web

rm -f pi-web.out

nohup ./pi-web $CONFIG_FILE > pi-web.out 2>&1 &
