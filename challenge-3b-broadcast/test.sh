#!/bin/bash

maelstrom_path="../maelstrom"  
cwd=$(pwd)

go build -o bin
cd "$maelstrom_path" || exit
./maelstrom test -w broadcast --bin $cwd/bin --node-count 5 --time-limit 20 --rate 10
cd "$cwd" || exit
