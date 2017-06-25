#!/bin/sh

cd /home/aaron/gowork/src/github.com/aaronriekenberg/pi-web

if [ $(uname) = 'OpenBSD' ]; then
  pgrep pi-web > /dev/null
  if [ $? -eq 1 ]; then
    ./restart.sh > /dev/null 2>&1
  fi
else
  echo 'not implemented'
  exit 1
fi
