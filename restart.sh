#!/bin/bash

killall pi-web

rm -f nohup.out

nohup ./pi-web ./config.yml &
