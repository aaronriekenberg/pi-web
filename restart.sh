#!/bin/sh

KILL_CMD=pkill
CONFIG_FILE=configfiles/$(hostname -s)-config.json

$KILL_CMD pi-web

sleep 2

export PATH=${HOME}/bin:$PATH

if [ -r output ]; then
  mv output output.prev
fi

nohup ./pi-web $CONFIG_FILE > output 2>&1 &
