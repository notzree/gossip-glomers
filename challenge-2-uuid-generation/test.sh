#!/bin/bash

cwd=$(pwd)

go build -o bin/unique-ids
maelstrom test -w unique-ids --bin $cwd/bin/unique-ids --time-limit 30 --rate 1000 --node-count 3 --availability total --nemesis partition
