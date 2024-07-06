# pe-terminal

[![License](https://img.shields.io/:license-apache-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/PelionIoT/pe-terminal)](https://goreportcard.com/report/github.com/PelionIoT/pe-terminal)

Terminal-client for Izuma Edge Gateways, (formerly [relay-term](https://github.com/PelionIoT/edge-node-modules/tree/master/relay-term)).
NOTE! Requires in minmimum `go` version 1.20.

## How to

- **Run:** To start terminal, do:
  ```bash
  ./make.sh run -config=example-config.json # or provide your own config.json
  ```
- **Build:** To generate the terminal binary, do:
  ```bash
  ./make.sh build
  ```
  or to cross-compile for another platform, do:
  ```bash
  ./make.sh build GOOS=<linux/mac> GOARCH=<amd64/arm> # windows support is not tested
  ```
- **Test:** To run unit tests, do:
  ```bash
  ./make.sh test
  ```
  or to run test in verbose-mode, do:
  ```bash
  ./make.sh test -v
  ```
- **Remove:** To remove generated binary, do:
  ```bash
  ./make.sh remove
  ```
- **Describe:** To view info about generated binary, do:
  ```bash
  ./make.sh describe
  ```

## License
----------
Apache 2.0. See the [LICENSE](https://github.com/PelionIoT/pe-terminal/blob/master/LICENSE) file for details.
