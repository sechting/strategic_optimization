#!/usr/bin/env python3

from __future__ import annotations
from typing import Iterable

def _clean_selector(sel: str) -> bytes:
    s = sel.lower().strip()
    if s.startswith("0x"): s = s[2:]
    if len(s) != 8 or any(c not in "0123456789abcdef" for c in s):
        raise ValueError(f"bad selector '{sel}' (need 8 hex chars)")
    return bytes.fromhex(s)

def build_init_and_dispatcher(selectors: Iterable[tuple[str, str]],) -> tuple[bytearray,list[tuple[int,str]]]:
    """
    Build init and dispatcher.

    selectors: iterable of (selector_hex, entry_label).
           entry_label is a symbolic label that will be resolved later.

    dispatcher layout (pseudocode):
        dup1
        push4 selector
        eq
        push2 jumpaddr
        jumpi

    We emit PUSH2 0x0000 for each jump target and return a 'patches' list
    with the byte offsets of those immediates so the real PCs can be filled in later.

    patches: [(offset:int, label:str), ...]
      - offset points to the FIRST byte of the 2-byte immediate (inside PUSH2).
      - label are exactly the passed hashes.
    """

    buf = bytearray()
    # standard init for POC this works for both erc20 and erc721
    buf.extend(bytes([0x60,0x80,0x60,0x40,0x52,0x34]))      # PUSH1 0x80, PUSH1 0x40, MSTORE, CALLVALUE
    buf.extend(bytes([0x80,0x15,0x61,0x00,0x0f,0x57]))      # DUP1, ISZERO, PUSH2 0x0f, JUMPI
    buf.extend(bytes([0x5f,0x5f,0xfd,0x5b,0x50,0x60,0x04])) # PUSH0, PUSH0, REVERT, JUMPDEST, POP, PUSH1 0x04
    buf.extend(bytes([0x36,0x10,0x61,0xff,0x57,0x5f]))      # CALLDATASIZE, LT, PUSH2 0xff, JUMPI 0xff is gonna be changed later
    buf.extend(bytes([0x5f,0x35,0x60,0xe0,0x1c]))           # PUSH0, CALLDATALOAD, PUSH1 0xe0, SHR
    patches: list[tuple[int, str]] = []

    # Create the dispatcher: the selectors list is ordered, so we just iterate through
    for sel_hex, label in selectors:
        sel4 = _clean_selector(sel_hex)
        buf.extend(bytes([0x80]))                      # DUP1
        buf.extend(bytes([0x63]) + sel4)               # PUSH4 <selector>
        buf.extend(bytes([0x14]))                      # EQ
        patches.append((len(buf) + 1, label))
        buf.extend(bytes([0x61, 0x00, 0x00]))          # PUSH2 <placeholder>
        buf.extend(bytes([0x57]))                      # JUMPI

    buf[22] = len(buf)                                 # fix the first JUMPI dest to jump to the REVERT location
    # append after the dispatcher the revert block, if a garbage function was called, then we default into this
    buf.extend(bytes([0x5b, 0x5f, 0x5f, 0xfd]))        # JUMPDEST, PUSH0, PUSH0, REVERT

    return (buf, patches)
