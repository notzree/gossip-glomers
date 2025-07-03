#!/bin/bash

cwd=$(pwd)

go build -o bin/broadcast
maelstrom test -w broadcast --bin $cwd/bin/broadcast --node-count 5 --time-limit 20 --rate 10
