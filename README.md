pe-terminal
===========
Terminal-client for Pelion-Edge Gateways, ( formerly [relay-term](https://github.com/PelionIoT/edge-node-modules/tree/master/relay-term) ).

How to
------
 * **Build:** To generate the terminal binary, do:
    ```bash
    ./make build
    ```

 * **Run:** To start terminal with paramters, do:
    ```bash
    ./make run -host=gateways.local -port=8080 -endpoint=/relay-term
    ```
    or to run with default parameters, do
    ```bash
    ./make run
    ```
 * **Remove:** To remove generated binary, do:
    ```bash
    ./make remove
    ```