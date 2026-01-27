#!/usr/bin/env python3
"""
composer.py

Step 1: Build a composition plan (KV-based)
  From the KV map, decide *which selectors* come from which optimization tier
  (HIGH vs LOW), sort dispatcher order by (priority, selector), and
  return a plan for downstream steps.

Downstream (not implemented here):
  2) Extract per-function blobs from chosen binaries (tier-specific), replace
     jump offsets with placeholders (A01, A02, ...)
  3) Recombine blobs into one image (still placeholders)
  4) Rebuild dispatcher and resolve placeholders

API
---
compose(binary_filename: str, kv: dict[str, dict]) -> int
    - binary_filename: name of the binary file to be used
    - kv: key-value map of selectors to their metadata

    returns: exit code (0 for success (new binary code in bin/finished/<binary_filename>), non-zero for failure)

"""
from __future__ import annotations

from pathlib import Path
from typing import Dict, List, Tuple, Literal
from lib.function_blobber import find_functions, load_runtime
from lib.dispatcher_bootstrap import build_init_and_dispatcher
from lib.deployment import create_deployment_code

Tier = Literal["high", "low"]

def _normalize_tier(opt: str) -> Tier:
    return "high" if str(opt).lower() == "h" else "low"


def _validate_kv(kv: Dict[str, Dict[str, object]]) -> Tuple[List[str], List[str]]:
    """
    From {selector: {priority,opt}}, build two lists of selectors (high/low),
    """
    high: List[str] = []
    low:  List[str] = []

    for sel_raw, meta in kv.items():
        sel = str(sel_raw).lower().strip()
        # must be 8 hex chars (function selector)
        if len(sel) != 8 or any(c not in "0123456789abcdef" for c in sel):
            continue

        tier = _normalize_tier(str(meta.get("opt", "L")))
        if tier == "high":
            high.append(sel)
        else:
            low.append(sel)

    return high, low



def compose(kv: Dict[str, Dict[str, object]]) -> Tuple[bytearray, bytearray]:
    """
    Step 1: produce a composition plan from a KV dict of selectors -> {priority,opt}.
            -> basically two lists of selectors (high/low)
    Step 2: extract per-function blobs from chosen binaries (tier-specific), replace
            jump offsets with placeholders (or incorrect addresses).
            every blob contains a label list in which the pc of the blob where the push2
            instruction is, with the target blob hash and an offset location inside the
            target blob. (pc this blob, target_blob_hash, target_blob_offset)
    Step 3: Recombine blobs into one image (still with placeholders)
    """
    high_list, low_list = _validate_kv(kv)
    print(f"Optimized selectors for runtime cost: {high_list}")
    print(f"Optimized selectors for deployment cost: {low_list}")

    func_starts = []
    if high_list:
        table_high = find_functions(load_runtime(Path("build/high/runtime.bin")), high_list, dict(), func_starts)
    else:
        table_high = {}
    if low_list:
        table_low = find_functions(load_runtime(Path("build/low/runtime.bin")), low_list, dict(), func_starts)
    else:
        table_low = {}

    #print(f"DEBUG:\ntable_high: {len(table_high)}\ntable_low: {len(table_low)}")
#    print(f"Function start PCs: {func_starts}")
#    print(f"kv: {kv}")

    # kv is already in the desired order
    order_index = {sel: i for i, sel in enumerate(kv.keys())}

    # sort in-place (unknown selectors go to the end)
    func_starts.sort(key=lambda t: order_index.get(t[0], float('inf')))
    #print(func_starts)

    (final_contract, patches) = build_init_and_dispatcher(func_starts)
    #print(f"dispatcher_blob: {final_contract.hex()}")
    
    # create a new table and add the entries missing from the table_low. we do not want duplicates!
    final_table = table_high | {k: v for k, v in table_low.items() if k not in table_high}
    #print(f"final_table length: {len(final_table)}")
    #for h, (_,_,code,_) in final_table.items():
        #print(f"Function 0x{h} code: {code.hex()}")

    for h, (start,end,blob,labels) in final_table.items():
        #print(f"Function 0x{h} @ {start}-{end} ({end-start} bytes)")
        # overwrite start as the new start location in the final_contract so we can edit the labels afterwards
        start = len(final_contract)
        final_contract.extend(blob)
        end = len(final_contract)-1
        final_table[h] = (start, end, blob, labels)
        #print(f"blob: {blob.hex()}")
        
    #print("\n")
    #for h, (start, end, _,_) in final_table.items():
        #print(f"Function 0x{h} @ {start}-{end} ({end-start} bytes)")
        #print(f"blob: {final_contract[start:end+1].hex()}")

    #print(f"final contract: {final_contract.hex()}")
    kv_blobpositions = {}
    for h,(start,end,code,_) in final_table.items():
        kv_blobpositions[h] = start

    # insert the correct jump locations in the bytecode, each label contains the pc of the current blob where
    # the push2 instruction is, the blob hash of the target and an offset inside the target blob.
    for _,(start,end,code,labels) in final_table.items():
        for (pc,label,offset)in labels:
            if kv_blobpositions.get(label) is None:
                print(f"we don't have blob position for label: {label}")
            #print(f">>>Patching position: {pc+offset} with 0x{kv_blobpositions[label]:x}-- 0x{kv_blobpositions[label].to_bytes(2, 'big').hex()} label:{label}")
            #print(f"{final_contract[start:end].hex()}")
            final_contract[start+pc+1:start+pc+3] = (kv_blobpositions[label]+offset).to_bytes(2, 'big')
            #print(f"setting new jump location at 0x{(start+pc+1):x} to 0x{(kv_blobpositions[label]+offset):x} offset is: 0x{offset:x}")
            #print(f"{final_contract[start:end].hex()}")

    #print(f"funcstarts: {func_starts}")
    #print(f"patches: {patches}")
    for pos, h in patches:
        #print(f"Patch at {pos}: {final_contract[pos-1:pos+2].hex()} -> 0x{kv_blobpositions[h]:x}")
        final_contract[pos:pos+2] = kv_blobpositions[h].to_bytes(2, 'big')

    # add metadata information for completion
    # 10 bytes of metadata: "solc": 0x00081d
    # a1 (map1) 64 text(4) 73 6f 6c 63 ("solc") 43 (bytes(3)) 00 08 1d (00081d solc version) 00 0a (10 bytes of metadata)
    final_contract.extend(bytes([0xa1, 0x64, 0x73, 0x6f, 0x6c, 0x63, 0x43, 0x00, 0x08, 0x1d, 0x00, 0x0a]))

    runtime_contract = bytearray()
    runtime_contract.extend(final_contract)
    runtime_len = len(runtime_contract)
    #print(f"runtime contract length: 0x{runtime_len:x}")

    # prepend the deployment code, which we will just copy from the existing compiled versions.
    final_contract = create_deployment_code(runtime_len) + final_contract
    return (final_contract, runtime_contract)
