#!/usr/bin/env python3
"""
DSL rules:
  - Example:  @dispatch(0) @opt function transfer(address to, uint256 value) public returns (bool) { ... }
  - @dispatch(<int>)  -> explicit priority
  - @opt              -> high optimization (H); if absent => low (L)
  - If function is public but missing @dispatch: -> default priority = 100
  - Only consider public functions
  - Print ordered by (priority, keccak(signature) lexicographically)

Notes:
  - We require the DSL tokens to be on the SAME line before 'function'.
  - Keccak-256: tries eth_utils.keccak, falls back to pysha3 (sha3.keccak_256).
"""

import re
from pathlib import Path
from typing import List, Dict, Any

# ---- Keccak support ---------------------------------------------------------

def _load_keccak():
    try:
        # pip install eth-utils
        from eth_utils.crypto import keccak as _keccak  # type: ignore
        return lambda b: _keccak(b)
    except Exception:
        pass
    try:
        # pip install pysha3
        import sha3  # type: ignore
        return lambda b: sha3.keccak_256(b).digest()
    except Exception:
        pass
    raise RuntimeError(
        "Keccak-256 not available. Install one of: 'eth-utils' or 'pysha3' (package name 'sha3')."
    )

keccak = _load_keccak()

# ---- Regexes ----------------------------------------------------------------

# Find every 'function NAME(' occurrence (works across the whole file)
FUNC_START = re.compile(r"function\s+(?P<name>[A-Za-z_]\w*)\s*\(", re.MULTILINE)

# Public visibility must appear before the first '{' after 'function'
OPEN_BRACE = re.compile(r"\{")

# ---- Helpers ----------------------------------------------------------------

# Helpers to read the current source line given an absolute index
def _line_bounds(text: str, idx: int) -> tuple[int, int]:
    """Return (line_start_idx, line_end_idx_exclusive) containing idx."""
    ls = text.rfind("\n", 0, idx) + 1  # start of line (0 if not found)
    le_n = text.find("\n", idx)
    le = len(text) if le_n == -1 else le_n
    return ls, le

def _extract_param_segment(text: str, open_paren_idx: int) -> str:
    """
    Given the index of '(' in 'function name(', return the substring from
    that '(' up to and including the matching ')' (handles nested parentheses).
    If unmatched, fall back to reading until the first ')'.
    """
    depth = 0
    for i in range(open_paren_idx, len(text)):
        c = text[i]
        if c == "(":
            depth += 1
        elif c == ")":
            depth -= 1
            if depth == 0:
                return text[open_paren_idx : i + 1]
    # Fallback: best effort
    j = text.find(")", open_paren_idx)
    return text[open_paren_idx : (j + 1 if j != -1 else len(text))]

_WORD = re.compile(r"\s+")
def _normalize_type(t: str) -> str:
    """
    Normalize Solidity type tokens for canonical signature:
    - collapse whitespace
    - drop 'payable' on address
    - convert 'uint' -> 'uint256', 'int' -> 'int256'
    - drop storage/location keywords: memory|calldata|storage
    - keep array suffixes [] as-is
    """
    t = _WORD.sub(" ", t.strip())
    # strip storage/location markers
    t = re.sub(r"\b(memory|calldata|storage)\b", "", t)
    t = t.replace("  ", " ").strip()

    # handle address payable -> address
    t = re.sub(r"\baddress\s+payable\b", "address", t)

    # grab base + array suffixes (e.g., "uint[][3]")
    m = re.match(r"^([A-Za-z0-9_]+(?:\s+[A-Za-z0-9_]+)?)\s*(\[.*\])?$", t)
    if m:
        base, arr = m.group(1), (m.group(2) or "")
    else:
        base, arr = t, ""

    base = base.strip()

    # uint/int shorthands
    if base == "uint":
        base = "uint256"
    elif base == "int":
        base = "int256"

    # collapse any remaining inner spaces in base (e.g., "fixed mx" edge cases)
    base = base.replace(" ", "")

    return f"{base}{arr}"

def _canonical_signature(name: str, params_inside_parens: str) -> str:
    """
    Build 'name(type1,type2,...)' with types only, canonicalized.
    """
    if params_inside_parens.strip() == "":
        return f"{name}()"
    parts = []
    # split on commas not inside tuples: assume no nested tuples yet
    # (If you need full tuple support later, we can upgrade this.)
    for part in params_inside_parens.split(","):
        # take the leading type token(s) before a variable name
        # heuristic: first token that looks like an identifier that's NOT an array suffix
        tok = part.strip()
        # drop var name by removing the last identifier if we have >1 tokens
        tokens = tok.split()
        if len(tokens) > 1:
            # keep everything except the last token (likely the var name)
            type_guess = " ".join(tokens[:-1])
        else:
            type_guess = tokens[0] if tokens else ""
        parts.append(_normalize_type(type_guess))
    return f"{name}({','.join(parts)})"

def _selector_hex_from_signature(sig: str) -> str:
    """
    Return the 4-byte selector as 8 hex chars (lowercase, no 0x).
    """
    return keccak(sig.encode("utf-8"))[:4].hex()

# --- sanitize DSL annotations (length-preserving) ----------------------------
def sanitize_annotations(src: str) -> str:
    """
    Replace @dispatch(n), and @opt tokens with spaces.
    Newlines are preserved so line numbers stay intact.
    """
    def blank(m: re.Match) -> str:
        s = m.group(0)
        # keep newlines intact; blank everything else
        return "".join("\n" if c == "\n" else " " for c in s)

    # New form: @dispatch(123)
    src = re.sub(r"(?<!\w)@dispatch\s*\(\s*\d+\s*\)",
                 blank, src, flags=re.IGNORECASE)

    # @opt token (no colon)
    src = re.sub(r"(?<!\w)@opt(?!\w)",
                 blank, src, flags=re.IGNORECASE)

    return src

def _is_public_before_brace(text_from_function: str) -> bool:
    m = OPEN_BRACE.search(text_from_function)
    head = text_from_function if not m else text_from_function[: m.start()]
    return re.search(r"\bpublic\b", head) is not None

def _is_external_before_brace(text_from_function: str) -> bool:
    m = OPEN_BRACE.search(text_from_function)
    head = text_from_function if not m else text_from_function[: m.start()]
    return re.search(r"\bexternal\b", head) is not None

def parse_file(path: Path) -> List[Dict[str, Any]]:
    src = path.read_text(encoding="utf-8", errors="replace")
    items: List[Dict[str, Any]] = []

    for m in FUNC_START.finditer(src):
        name = m.group("name")
        # Build a forward window to include modifiers until '{'
        start = m.start()
        # Find line start that contains the 'function' token (captures prefix of the line)
        line_start, _ = _line_bounds(src, start)
        # Find the first '{' after 'function' to cap the declaration region
        mbrace = OPEN_BRACE.search(src, start)
        end_idx = mbrace.start() if mbrace else min(len(src), start + 8000)

        # declaration region = from the beginning of this line up to (but not including) '{'
        decl_region = src[line_start:end_idx]
        if not _is_public_before_brace(decl_region) and not _is_external_before_brace(decl_region):
            print(f"skipping function '{name}': not public or is external {_is_external_before_brace(decl_region)}")
            continue  # skip non-public or non external functions

        prio = None
        m_dispatch = re.search(r"(?<!\w)@dispatch\s*\(\s*(\d+)\s*\)", decl_region, re.IGNORECASE)
        if m_dispatch:
            prio = int(m_dispatch.group(1))
 
        # @opt => high optimization if present anywhere before '{'
        high_opt = bool(re.search(r"(?<!\w)@opt(?!\w)", decl_region, re.IGNORECASE))
        opt = "H" if high_opt else "L"

        # Default priority if omitted but public
        if prio is None:
            prio = 100

        # Build signature text for hashing (deterministic ordering tie-breaker)
        # Take function name + the exact parameter segment as written.
        open_paren_idx = m.end() - 1  # index of '('
        params_seg = _extract_param_segment(src, open_paren_idx)  # e.g., "(address to, uint value)"
        params_inside = params_seg[1:-1]  # drop surrounding parentheses

        sig_canonical = _canonical_signature(name, params_inside)  # e.g., "transfer(address,uint256)"
        selector_hex = _selector_hex_from_signature(sig_canonical)  # first 4 bytes, 8 hex chars

        items.append(
            {
                "name": name,
                "priority": prio,
                "opt": opt,  # 'h' or 'l'
                "signature": sig_canonical,
                "selector": selector_hex,
            }
        )

    return items