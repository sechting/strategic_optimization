.PHONY: erc20_scanner preprocess cpu_perfunc dispatcher_waste traceblock generate all

# Optional args (override like: make erc20scanner START=... END=...)
START ?=
END   ?=
CORES ?=
CHUNK ?=

PPROC ?=

FILE ?=

# Build up CLI flags only if variables are set
ERC20_FLAGS :=
ifneq ($(strip $(START)),)
  ERC20_FLAGS += --start=$(START)
endif
ifneq ($(strip $(END)),)
  ERC20_FLAGS += --end=$(END)
endif
ifneq ($(strip $(CORES)),)
  ERC20_FLAGS += --cores=$(CORES)
endif
ifneq ($(strip $(CHUNK)),)
  ERC20_FLAGS += --chunk=$(CHUNK)
endif

PPROC_FLAGS :=
ifneq ($(strip $(PPROC)),)
  PPROC_FLAGS += --file=$(PPROC)
endif

TRACE_FLAGS :=
ifneq ($(strip $(START)),)
  TRACE_FLAGS += --start=$(START)
endif
ifneq ($(strip $(END)),)
  TRACE_FLAGS += --end=$(END)
endif
ifneq ($(strip $(CORES)),)
  TRACE_FLAGS += --cores=$(CORES)
endif

GENERATE_FLAGS :=
ifneq ($(strip $(FILE)),)
  GENERATE_FLAGS += FILE=$(FILE)
endif
all:
	$(MAKE) generate
	go run ./cmd/erc20_scanner $(ERC20_FLAGS)
	go run ./cmd/preprocess $(PPROC_FLAGS)
	go run ./cmd/cpu_perfunc
	go run ./cmd/dispatcher_waste
	go run ./cmd/traceblock

erc20_scanner:
	go run ./cmd/erc20_scanner $(ERC20_FLAGS)

preprocess:
	go run ./cmd/preprocess $(PPROC_FLAGS)

cpu_perfunc:
	go run ./cmd/cpu_perfunc

dispatcher_waste:
	go run ./cmd/dispatcher_waste

traceblock:
	go run ./cmd/traceblock $(TRACE_FLAGS)

generate:
	$(MAKE) -C contract_generation run $(GENERATE_FLAGS)

clean:
	$(MAKE) -C contract_generation clean
	rm -rf data/*