#!/usr/bin/env python3

from __future__ import annotations

import sys

from pathlib import Path
from lib.parser import parse_file, sanitize_annotations
from lib.compiler import compile_sanitized_with_opt
from lib.composer import compose

def main(argv: list[str]) -> int:
    if len(argv) != 2:
        print(f"Usage: {argv[0]} <file.sol>", file=sys.stderr)
        return 2
    p = Path(argv[1])
    if not p.exists():
        print(f"File not found: {p}", file=sys.stderr)
        return 2

    # ---- parsing and sanitizing ---------------------------------------------

    items = parse_file(p)

    # Sort by (priority asc, keccak asc)
    items.sort(key=lambda x: (x["priority"], x["selector"]))

    if not items:
        print("(no public functions with DSL on the line or defaulted)")
        return 0
    
    # Print in order
    for it in items:
        print(f"{it['name']:>20} -> dispatch={it['priority']:3d}, opt={it['opt']}, keccak={it['selector']}")

    # If you want the key-value “save” shape, here it is as a dict:
    # func -> { priority, opt }
    kv = {f"{it['selector']}": {"priority": it["priority"], "opt": it["opt"]} for it in items}
    # Uncomment to view JSON:
    # import json; print(json.dumps(kv, indent=2))

    # ---- compilation --------------------------------------------------------

    # Do we need high and/or low builds?
    has_high = any(it["opt"].lower() == "h" for it in items)
    has_low  = any(it["opt"].lower() == "l" for it in items)

    # Write a sanitized copy for the compiler
    raw = p.read_text(encoding="utf-8", errors="replace")
    sanitized = sanitize_annotations(raw)
    san_path = p.with_suffix(".sanitized.sol")
    san_path.write_text(sanitized, encoding="utf-8")
    print(f"Sanitized source written to: {san_path}")

    exit_codes = []
    rc_h = compile_sanitized_with_opt("solc", san_path.name, sanitized, opt=True)
    rc_l = compile_sanitized_with_opt("solc", san_path.name, sanitized, opt=False)
    exit_codes += [rc_h, rc_l]

    if exit_codes[0] != 0:
        print(f"Compilation failed: {exit_codes}", file=sys.stderr)
        return max(exit_codes)
    
    contract, runtime = compose(kv)
    #print(f"Generated contract:\n{contract.hex()}")
    #print(f"Contract size: {len(contract)} bytes")
    (Path("build")/"final").mkdir(parents=True, exist_ok=True)
    (Path("build")/"final"/"deployment.bin").write_text(contract.hex(),encoding="utf-8")
    (Path("build")/"final"/"runtime.bin").write_text(runtime.hex(),encoding="utf-8")

    return max(exit_codes) if exit_codes else 0

if __name__ == "__main__":
    import sys
    raise SystemExit(main(sys.argv))

