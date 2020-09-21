#!/bin/sh

KILL_CMD=pkill
CONFIG_FILE=config/$(hostname -s)-config.json

$KILL_CMD pi-web

sleep 2

export PATH=${HOME}/bin:$PATH

if [ -r nohup.out ]; then
  mv nohup.out nohup.prev.out
fi

nohup ./pi-web $CONFIG_FILE &
