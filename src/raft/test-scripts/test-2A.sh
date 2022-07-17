#!/bin/zsh

cores=$(grep -c processor /proc/cpuinfo)

cd ../ && bash go-race-many.sh -t 2A -r 3000 -p "$((cores * 4))" -c -race
