#!/bin/sh

pgrep pi-web > /dev/null 2>&1
if [ $? -eq 1 ]; then
  if [ $(uname) = 'OpenBSD' ]; then
    cd /home/aaron/go/src/github.com/aaronriekenberg/pi-web
    ./restart.sh > /dev/null 2>&1
  else
    cd /home/pi/gowork/src/github.com/aaronriekenberg/pi-web
    ./restart.sh > /dev/null 2>&1
  fi
fi
