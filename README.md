# strategic_optimization
**Paper:** Sustainable Smart Contract Execution via Usage-Aware Strategic Optimization *(submitted to ICBC'26)*

## Project Structure
```
├── LICENSE
├── Makefile
├── README.md
├── cmd/
│   ├── cpu_perfunc
│   ├── dispatcher_waste
│   ├── erc20_scanner
│   ├── preprocess
│   └── traceblock
├── contract_generation/
│   ├── build/
│   ├── src/
│   └── Makefile
├── data/
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

## Strategic Optimizer (Contract Generation)

**WARNING** this is only a research grade tool, it works with the given ERC20 and ERC721 contracts. Other contracts might not work straight out of the box.

#### Compiling, recomposing and altering function dispatcher

This tools principles should ideally be applied in the Solidity compiler. If you search carefully (or not even that carefully),
you'll find flaws. No biggie! This tool can be developed further for your own sake. Feel free to contribute if you wish!

When trying to compile and recompose a smart contract, you need to have all solidity files that are
needed in the `src/` directory. To avoid screwing things up, use the Makefile and pass the file
that should be compiled:

`make run FILE=src/file.sol`

this will automatically set up the venv for python and then compile the provided contract. Output will
be three deployable contracts in `build/` one in `low/`, one in `high/` and the recomposed contract in
`final/`. you can test the `runtime.bin`, which is the deployed contract in your preferred playground, or
deploy the `deployment.bin` either in your test network or go live with it. But please test it thoroughly
before anything.

In case the venv has been messed up manually, you can remove it by using:

`make clean-venv`

And rebuild it by just running a:

`make venv` or simply call `make run` and it will create the venv again automatically.

#### Testing

For testing, Anvil from Foundry was used; install it like this:

`curl -L https://foundry.paradigm.xyz | bash`

`foundryup`

Start anvil with this simple command in the cli:

`anvil`

You might want better debugging output with:

`anvil --steps-tracing --print-traces`

To evaluate the given test contracts, you can use the provided test scripts:

`test/token.test.mjs` or `test/nft.test.mjs`

With these scripts you can test the integrity of the token and NFT and get some numbers, which will be saved
in `poc_results.tex` or `poc_nftresults.tex` (which are the numbers used in the paper). Use Node for the scripts:

`node test/token.test.mjs` or `node test/nft.test.mjs` respectively.
