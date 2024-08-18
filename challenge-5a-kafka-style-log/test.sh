#!/bin/bash

maelstrom_path="../maelstrom"  
cwd=$(pwd)

go build -o bin
cd "$maelstrom_path" || exit
./maelstrom test -w kafka --bin $cwd/bin --node-count 1 --concurrency 2n --time-limit 20 --rate 1000
cd "$cwd" || exit
