# strategic_optimization
**Paper:** Sustainable Smart Contract Execution via Usage-Aware Strategic Optimization *(submitted to ICBC'26)*

## Project Structure
```
├── LICENSE
├── Makefile
├── README.md
├── cmd
│   ├── erc20_scanner
│   └── traceblock
├── data
│   └── generated(erc20scrape.csv)
├── go.mod
└── go.sum
```
## Measurement Study
First step, collect all the erc20 data using the erc20_scanner, there are four parameters that can be
passed, all are optional. ``START``, ``END``, ``CORES`` and ``CHUNK``.
example:

``make erc20_scanner START=912760 END=920000 CORES=16 CHUNK=10000``

``START`` denotes the first block that should be scraped for erc20 contracts, if omitted it defaults to
912760 (the first official erc20 contract)

``END`` specifies the last block that is still being processed, if omitted it defaults to 912761

``CORES`` declares the amount of threads that should be utilized for parallelization, if omitted it defaults to 8

``CHUNK`` denotes the chunksize when processing the batches concurrently, if omitted it defaults to 100,000

This will result in the file ``erc20scrape.csv`` in the ``data`` directory which is the basis for the following analysis scripts.