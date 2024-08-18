#!/bin/bash

maelstrom_path="../maelstrom"  
cwd=$(pwd)

go build -o bin
cd "$maelstrom_path" || exit
./maelstrom test -w g-counter --bin $cwd/bin --node-count 3 --rate 100 --time-limit 20 
cd "$cwd" || exit
