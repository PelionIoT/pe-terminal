#!/bin/bash

# Builds binary of pe-terminal
build() {
    remove
    echo "Building pe-terminal..."
    if [[ -n "$1" && -n "$2" ]]; then
        # Expected parameters GOOS=linux/mac/windows GOARCH=amd64/arm
        env "$1" "$2" go build -v .
    else
        go build -v .
    fi
}

# Starts pe-terminal with/without parameters
run() {
    if [[ -n "$1" ]]; then
        build "$@"
        ./pe-terminal "$1"
    else
        echo "No config-file provided, use flag -config=<filename>.json"
    fi
}

# Runs all the unit tests
test() {
    go vet
    if [[ -n "$1" ]]; then
        go test "$1" -timeout 15s github.com/PelionIoT/pe-terminal/components
    else
        go test -timeout 15s github.com/PelionIoT/pe-terminal/components
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
