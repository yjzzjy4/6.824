#!/bin/zsh

cores=$(grep -c processor /proc/cpuinfo)

cd ../ && bash go-race-many.sh -t 2D -r 3000 -p "$((cores * 3))" -c -race