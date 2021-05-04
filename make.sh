#!/bin/bash

# Builds binary of pe-terminal
build() {
    remove
    go build .
}

# Starts pe-terminal with/without parameters
run() {
    build
    if [[ -n "$1" && -n "$2" && -n "$3" ]]; then
        ./pe-terminal $1 $2 $3
    elif [[ -n "$1" && -n "$2" ]]; then
        ./pe-terminal $1 $2
    elif [[ -n "$1" ]]; then
        ./pe-terminal $1
    else
        ./pe-terminal
    fi
}

# Removes binary of pe-terminal
remove() {
    rm -rf pe-terminal
}

"$@"