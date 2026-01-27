#!/usr/bin/env python3

import json
import subprocess
import sys
from pathlib import Path

# --- compile sanitized source with solc --------------------------------------
MAX_RUNS = (1 << 32) - 1
BUILD_DIR = Path("build")

def compile_sanitized_with_opt(solc_path: str, file_label: str, sanitized_src: str, opt: bool) -> int:
    """
    Compile a single sanitized source string using solc --standard-json:
      - optimizer enabled with runs = 2^32 - 1
      - metadata hash disabled (no metadata hash appended to bytecode)
    Writes the raw solc JSON output to build/<tier>/
    Returns the solc exit code.
    """
    runs = MAX_RUNS if opt else 0
    tier = "high" if opt else "low"
    out_dir = BUILD_DIR / tier
    out_dir.mkdir(parents=True, exist_ok=True)

    payload = {
        "language": "Solidity",
        "sources": {
            "src/" + file_label: {"content": sanitized_src}
        },
        "settings": {
            "optimizer": {"enabled": True, "runs": runs},
            "metadata": {"bytecodeHash": "none"},
            "outputSelection": {"*": {"*": ["abi", "evm.bytecode", "evm.deployedBytecode", "metadata", "devdoc", "userdoc"]}},
        },
    }

    proc = subprocess.run(
        [solc_path, "--standard-json"],
        input=json.dumps(payload).encode("utf-8"),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )

    out_json_path = out_dir / f"{file_label}.out.json"
    out_json_path.write_bytes(proc.stdout)

    if proc.returncode != 0:
        # still useful to keep the JSON; print stderr to help during POC
        print(proc.stderr.decode("utf-8", errors="ignore"), file=sys.stderr)

    # Emit .abi/.bin into build/<tier>/
    try:
        stdjson = json.loads(proc.stdout.decode("utf-8"))
        _write_artifacts("src/"+file_label,stdjson, out_dir)
    except Exception as e:
        print(f"[warn] failed to write artifacts: {e}", file=sys.stderr)

    return proc.returncode

def _write_artifacts(target: str, stdjson: dict, out_root: Path) -> None:
    """
    Write <Contract>.abi and <Contract>.bin into out_root from solc standard-json output.
    """
    contracts = stdjson.get("contracts", {})
    out_root.mkdir(parents=True, exist_ok=True)

    for _file, per_file in contracts.items():
        if _file != target:
            continue
        for contract_name, artefacts in per_file.items():
            bytecode = artefacts.get("evm", {}).get("bytecode", {}).get("object", "")
            if not bytecode:
                # bin and bin-runtime will be empty, abi is just junk at this point.
                continue

            abi = artefacts.get("abi", [])
            runtime_obj = artefacts.get("evm", {}).get("deployedBytecode", {}).get("object", "")
            (out_root / f"{contract_name}.abi").write_text(
                json.dumps(abi, indent=2), encoding="utf-8"
            )
            (out_root / f"deployment.bin").write_text(  # deployment code
                bytecode or "", encoding="utf-8"
            )
            (out_root / f"runtime.bin").write_text( # runtime code
                runtime_obj or "", encoding="utf-8"
            )    
