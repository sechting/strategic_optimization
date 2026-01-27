# Makefile for Solidity dispatcher pipeline

PYTHON := python3
VENV   := venv
PIP    := $(VENV)/bin/pip
PY     := $(VENV)/bin/python

# Default target
.DEFAULT_GOAL := help
FILE := src/testtoken.sol
## Create virtual environment and install deps
venv: $(VENV)/bin/activate

$(VENV)/bin/activate: requirements.txt
	$(PYTHON) -m venv $(VENV)
	$(PIP) install --upgrade pip
	$(PIP) install -r requirements.txt
	touch $(VENV)/bin/activate

## Clean build dir and remove all *.sanitized.sol files
clean:
	rm -rf build
	mkdir -p build
# remove sanitized files anywhere in the tree
	find . -type f -name '*.sanitized.sol' -delete

## Run parser on a Solidity file, pass FILE=<path>
run: clean venv
	$(PY) generate.py $(FILE)

## Remove the virtualenv too (heavier cleanup)
clean-venv:
	rm -rf $(VENV)

## Show available targets
help:
	@echo "Targets:"
	@echo "  make venv          - create venv and install deps"
	@echo "  make clean         - fresh build dir and remove *.sanitized.sol"
	@echo "  make clean-venv    - remove the venv only"
	@echo "  make run  			- sanitize + compile with solc into build/"