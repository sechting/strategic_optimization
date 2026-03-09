# Strategic Optimization
**Paper:** Sustainable Smart Contract Execution via Usage-Aware Strategic Optimization *(submitted to ICBC'26)*


## Measurement Study
The easiest way to reproduce the measurement study is by running the `all` command via make. This will build the *testtoken*, scrape the archive node for all ERC-20 contracts and processes the data step by step automatically.

**Note:** the paper used the following command:
```
make all END=21525785 CORES=40
```

Below are all individual scripts described.

### erc20_scanner
First step, collect all the ERC-20 data using the **erc20_scanner**, there are four parameters that can be
passed, all are optional. `START`, `END`, `CORES` and `CHUNK`.
example:

```
make erc20_scanner START=912760 END=920000 CORES=16 CHUNK=10000
```

`START` denotes the first block that should be scraped for erc20 contracts, if omitted it defaults to
912760 (the first official ERC-20 contract)

`END` specifies the last block that is still being processed, if omitted it defaults to 912761

`CORES` declares the amount of threads that should be utilized for parallelization, if omitted it defaults to 8

`CHUNK` denotes the chunksize when processing the batches concurrently, if omitted it defaults to 100,000

This will result in the file `erc20scrape.csv` in the `data/` directory which is the basis for the following analysis scripts.

### preprocess
The initial scrape needs processing via the `preprocess` script. It will load the `erc20scrape.csv` table by default, but it can be directed towards another csv file **Note:** the whole path from root directory has to be passed.

`PPROCFILE` specifies the csv table that should be used instead of `data/erc20scrape.csv`

**Example:**
```
make preprocess PPROCFILE= data/alternate.csv
```

This results in different directories and json files that will be used further down the pipeline for evaluation. Specifically:
```
├── data
.   ├── contracts/
.   ├── dispatcher/
.   ├── init/
    ├── dispatcher_sequences.json
    ├── dispatcher_sequences_args.json
    ├── init_dispatcher_sequences.json
    └── init_sequences.json
```
Where directories contain unique sequences inside individual json files, for either whole contracts, init or dispatcher parts. `dispatcher_sequences.json` contains all unique normalized (evm instructions without arguments) dispatcher sequences coupled with the contracts that employ these dispatcher. The same holds for `init_sequences.json`. As the name suggests, `dispatcher_sequences_args.json` is a denormalized version still containing arguments. `init_dispatcher_sequences.json` combines the init and dispatcher parts to a single sequence and also contains the arguments. 

### cpu_perfunc
Filters the sequences inside `init_dispatcher_sequences.json` further, by removing sequences that are not executable in its current form (e.g. jumps into later parts of the code that were removed). Then the addresses of the contracts still being processed are saved in `data/scrape_addresses.json` for further extraction of call data throughout their lifecycle.

Main goal of this script is to calculate a cpu time estimation for each unique init_dispatcher_sequence until the point the desired function is reached (`balanceOf` and `transfer` this is a hen-egg problem, we need `scrape_addresses.json` to further scrape data about usage and will be able to decide which functions are relevant). Findings will be saved in `data/call_costs_{70a08231|a9059cbb}.csv` as total cpu time per function, and once again in `data/call_costs_diff_{70a08231|a9059cbb}.csv` as the difference with the proposed strategic optimized contract.

**Example:**
```
make cpu_perfunc
```
**Note:** the strategic optimized contract via `contract_generation` must be compiled an be ready in `contract_generation/build/{final|high}`

This results in additional files in `data/`:
```
├── data
.   ├── scrape_addresses.json
.   ├── call_costs_70a08231.csv
.   ├── call_costs_a9059cbb.csv
    ├── call_costs_diff_70a08231.csv
    └── call_costs_diff_a9059cbb.csv
```

### dispatcher_waste
Similar to `cpu_perfunc` this script will filter the sequences exactly the same and calculates the estimated cpu times and gas cost per function (balanceOf and transfer) but this time exclusively for the dispatcher, disregarding the initialization part of the contract via the `dispatcher_sequences_args.json` file. The output results in additional files in `data/`:
```
├── data
.   ├── selector_waste_70a08231,csv
.   └── selector_waste_a9059cbb.csv
```

**Example:**
```
make dispatcher_waste
```

### traceblock
Traces all blocks in the range defined in `START` and `END` and searches for transactions that call the contracts specified in `scrape_addresses.json`. Every function call found is then written to `erc20calls.csv` in the `data/` directory. To speed things up, `CORES` will be applied to increase the amount of threads used for the scrape.

**Example:**
```
make traceblock START=920000 END=940000 CORES=40
```

## Contract Generation

**WARNING** this is still a research grade tool, it works with the given ERC-20 and ERC-721 contracts. For other contracts tweaking might become necessary.

#### Compiling, recomposing and altering function dispatcher

This tool is still in experimental development. It serves as a POC, and its principles ideally
be applied in the Solidity compiler. If you search carefully (or not even that carefully), you'll find flaws.
No biggie! This tool can be developed further for your own sake. Feel free to contribute if you wish!

When trying to compile and recompose a smart contract, you need to have all solidity files that are
needed in the `src/` directory. To avoid screwing things up, use the Makefile and pass the file
that should be compiled:

```
make generate FILE=src/file.sol
```

It is possible to change into the `contract_generation` directory and invoke the Makefile there as well like this:

```
make run FILE=src/file.sol
```

this will automatically set up the venv for python and then compile the provided contract. Output will
be three deployable contracts in `build/` one in `low/`, one in `high/` and the recomposed contract in
`final/`. you can test the `runtime.bin`, which is the deployed contract in your preferred playground, or
deploy the `deployment.bin` either in your test network or go live with it. But please test it thoroughly
before anything.

In case the venv has been messed up manually, you can remove it by using inside the `contract_generation` directory:

```
make clean-venv
```

And rebuild it by just running a:

`make venv` or simply call `make run` and it will create the venv again automatically.

#### Testing

For testing, Anvil from Foundry was used; install it like this:

```
curl -L https://foundry.paradigm.xyz | bash
```

```
foundryup
```

Start anvil with this simple command in the cli:

```
anvil
```

You might want better debugging output with:

```
anvil --steps-tracing --print-traces
```

To evaluate the given test contracts, you can use the provided test scripts:

`test/token.test.mjs` or `test/nft.test.mjs`

With these scripts you can test the integrity of the token and NFT and get some numbers, which will be saved
in `poc_results.tex` or `poc_nftresults.tex`, which are the numbers used in the paper. Use Node for the scripts:

`node test/token.test.mjs` or `node test/nft.test.mjs` respectively.

## Plotting


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
│   ├── Makefile
│   ├── generate.py
│   ├── lib/
│   ├── requirements.txt
│   ├── src/
│   ├── test/
│   └── venv
├── data/
├── go.mod
├── go.sum
└── internal/
│   ├── cpu
│   ├── io
│   └── seq
└── plotting/
    ├── eval.py
    └── measure.py
```
