#!/bin/bash

cwd=$(pwd)

go build -o bin/g-counter
maelstrom test -w g-counter --bin $cwd/bin/g-counter --node-count 3 --rate 100 --time-limit 20 --nemesis partition
