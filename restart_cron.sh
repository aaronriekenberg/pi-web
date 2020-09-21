#!/bin/sh

pgrep pi-web > /dev/null 2>&1
if [ $? -eq 1 ]; then
  cd ~/pi-web
  ./restart.sh > /dev/null 2>&1
fi
