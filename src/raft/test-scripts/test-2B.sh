#!/bin/zsh

cores=$(grep -c processor /proc/cpuinfo)

cd ../ && bash go-race-many.sh -t 2B -r 3000 -p "$((cores * 2))" -c -race
