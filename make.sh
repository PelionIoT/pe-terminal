#!/bin/bash

# Builds binary of pe-terminal
build() {
    remove
    echo "Building pe-terminal..."
    if [[ -n "$1" && -n "$2" ]]; then
        # Expected parameters GOOS=linux/mac/windows GOARCH=amd64/arm
        env $1 $2 go build -v .
    else
        go build -v .
    fi
}

# Starts pe-terminal with/without parameters
run() {
    build
    if [[ -n "$1" && -n "$2" && -n "$3" ]]; then
        # Expected parameters -host=gateways.local -port=8080 -endpoint=/relay-term
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
    echo "Cleaning build-cache..."
    rm -rf pe-terminal
}

# Displays binary info
describe() {
    file pe-terminal
}

"$@"