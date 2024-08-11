#!/bin/bash

maelstrom_path="../maelstrom"  
cwd=$(pwd)

mkdir -p bin
go build -o bin
cd "$maelstrom_path" || exit
./maelstrom test -w broadcast --bin $cwd/bin --node-count 25 --time-limit 20 --rate 100 --latency 100
cd "$cwd" || exit
