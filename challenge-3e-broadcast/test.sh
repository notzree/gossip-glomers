#!/bin/bash

cwd=$(pwd)

go build -o bin/broadcast
maelstrom test -w broadcast --bin $cwd/bin/broadcast --node-count 25 --time-limit 20 --rate 100 --latency 100