#!/bin/sh

if [ $(uname) = 'OpenBSD' ]; then
  pgrep -q pi-web
  if [ $? -eq 1 ]; then
    cd /home/aaron/gowork/src/github.com/aaronriekenberg/pi-web
    ./restart.sh > /dev/null 2>&1
  fi
else
  pgrep pi-web > /dev/null 2>&1
  if [ $? -eq 1 ]; then
    cd /home/pi/gowork/src/github.com/aaronriekenberg/pi-web
    ./restart.sh > /dev/null 2>&1
  fi
fi
