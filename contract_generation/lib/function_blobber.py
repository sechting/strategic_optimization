#!/usr/bin/env python3
"""
function_blober.py

Current capabilities:
- Load runtime bytecode (hex) and emulate enough of the EVM to walk the
  dispatcher: PUSHn, DUPn, SWAPn, EQ, AND, SHR, JUMP, JUMPI, JUMPDEST,
  CALLDATALOAD, CALLDATASIZE, POP, STOP/RETURN/REVERT.
- For a given 4-byte selector, run from pc=0 with calldata = selector || zeros
  and return all blobs starting at first JUMPDEST of that function.
"""
from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Iterable
from collections import defaultdict
import hashlib

# ------------------------- I/O helpers --------------------------------------

def load_runtime(path: Path) -> bytes:
    s = path.read_text(encoding="utf-8", errors="ignore").strip()
    if s.startswith("0x") or s.startswith("0X"):
        s = s[2:]
    s = "".join(ch for ch in s if ch.strip() != "")
    if len(s) % 2:
        s = "0" + s
    return bytes.fromhex(s)


def preview_at(code: bytes, pc: int, nbytes: int = 10) -> str:
    if pc < 0 or pc >= len(code):
        return ""
    return code[pc : min(len(code), pc + nbytes)].hex()

# ------------------------- Minimal EVM --------------------------------------
U256 = (1 << 256) - 1


def _push_len(op: int) -> int:
    return op - 0x5F if 0x60 <= op <= 0x7F else 0


def _dup_n(op: int) -> int:
    return op - 0x7F if 0x80 <= op <= 0x8F else 0


def _swap_n(op: int) -> int:
    return op - 0x8F if 0x90 <= op <= 0x9F else 0

StackElem = tuple[int, int]  # (value, pc_when_pushed)
StackSig = tuple[StackElem, ...] # stacks signature when using worked segments
@dataclass
class VM:
    code: bytes
    calldata: bytes
    sel: int
    pc: int = 0
    # stack tuple (content, pc when stacked)
    stack: list[tuple[int,int]] = None  # type: ignore
    memory: bytearray = None  # type: ignore
    halted: bool = False
    reason: str = ""
    jumpfunction: bool = False
    blobs: list[tuple[int, int]] = None # type: ignore
    recording: bool = False
    branches: list[tuple[int, list[tuple[int, int]]]] = None # type: ignore
    worked_segments: defaultdict[int, set[StackSig]]= None # type: ignore
    # labels (push2 instr pc, jumpaddr)
    labels: list[tuple[int, int]] = None # type: ignore
    entry_pc: int = 0

    def dump_stack(self, max_items: int = 20, padded: bool = True) -> str:
        fmt = (lambda v, p: f"0x{v:x}(0x{p:x})") if padded else (lambda v, p: f"0x{v:x}(0x{p:x})")
        seg = self.stack[-max_items:]
        return "[" + " ".join(fmt(v, p) for v, p in reversed(seg)) + "]"  # top→bottom
    
    def dump_labels(self) -> str:
        fmt = (lambda v, p: f"pc:0x{v:x}->(0x{p:x})")
        seg = self.labels
        return "[" + " ".join(fmt(v, p) for v, p in seg) + "]"  # top→bottom

    def __post_init__(self) -> None:
        if self.stack is None:
            self.stack = []
        if self.memory is None:
            self.memory = bytearray()
        if self.blobs is None:
            self.blobs = []
        if self.branches is None:
            self.branches = []
        if self.labels is None:
            self.labels = []
        if self.worked_segments is None:
            self.worked_segments = defaultdict(set)

    def _push(self, v: int) -> None:
        self.stack.append((v & U256, self.pc -1))

    def _pop(self) -> tuple[int, int]:
        if not self.stack:
            return 0, 0
        return self.stack.pop()
    
    def _dequeue(self) -> tuple[int,list[tuple[int,int]]]:
        if not self.branches:
            return 0,[(0,0)]
        return self.branches.pop(0)

    def _dup(self, id) -> None:
        self.stack.append(self.stack[-id])
    
    def _start_blob(self, dest) -> None:
        self.blobs.append((dest, 0))
        self.recording = True
        #print(f"started recording blob at pc: 0x{dest:x}")

    def _end_blob(self) -> None:
        if self.blobs:
            self.blobs[-1] = (self.blobs[-1][0], self.pc)
            #print(f"ended recording blob at pc: 0x{self.pc:x}")
            #print(f"blobs: {self.blobs}")
    
    def step(self) -> None:
        """
        Execute a single instruction.
        Returns None. Sets halted on STOP/RETURN/REVERT.
        """

        #print(f"__vm__ 0x{self.pc:x} op:{self.code[self.pc]:x}")
        if self.pc < 0 or self.pc >= len(self.code):
            self.halted = True
            self.reason = "pc_oob"
            return None

        op = self.code[self.pc]
        self.pc += 1

        # STOP
        if op == 0x00:
            if self.recording:
                self.pc -= 1
                self._end_blob()
            if self.branches:
                # we have branches left to walk through
                self.pc, self.stack = self._dequeue()
                self._start_blob(self.pc)
                #print(f"resuming branch at pc: 0x{self.pc:x} stack: {self.dump_stack()}")
                #print(f"resuming with branch at pc: 0x{self.pc:x} total branches: {len(self.branches)}")
                #print(f"encountered end of code but blob is left at 0x{self.pc:x}")
                #print(f"resuming with stack: {self.dump_stack()}")
                return None
            self.halted = True
            self.reason = "stop"
            return None

        # RETURN / REVERT
        if op in (0xF3, 0xFD):
            if self.recording:
                self.pc -= 1
                self._end_blob()
                if self.branches:
                    # we have branches left to walk through
                    self.pc, self.stack = self._dequeue()
                    self._start_blob(self.pc)
                    #print(f"resuming branch at pc: 0x{self.pc:x} stack: {self.dump_stack()}")
                    #print(f"resuming with branch at pc: 0x{self.pc:x} total branches: {len(self.branches)}")
                    #print(f"encountered end of code but blob is left at 0x{self.pc:x}")
                    #print(f"resuming with stack: {self.dump_stack()}")
                    return None
            self.halted = True
            self.reason = "return" if op == 0xF3 else "revert"
            return None

        # JUMPDEST (no-op)
        if op == 0x5B:
            return None

        # PUSHn
        plen = _push_len(op)
        if plen:
            if self.pc + plen > len(self.code):
                self.halted = True
                self.reason = "push_oob"
                return None
            imm = int.from_bytes(self.code[self.pc : self.pc + plen], "big")
            self._push(imm)
            self.pc += plen
            #print(f"push{plen} <{imm:x}>")
            #print(self.dump_stack())
            return None
        elif op == 0x5F: # PUSH0
            self._push(0)
            return None

        # DUPn
        d = _dup_n(op)
        if d:
            #idx = -d
            if len(self.stack) >= d:
                self._dup(d)
            #self._push(self.stack[idx] if len(self.stack) >= d else 0)
            return None

        # SWAPn
        s = _swap_n(op)
        if s:
            a = -1
            b = -1 - s
            if len(self.stack) >= (s + 1):
                self.stack[a], self.stack[b] = self.stack[b], self.stack[a]
            return None
        
        # ADD
        if op == 0x01:
            a,_ = self._pop()
            b,_ = self._pop()
            self._push((a + b) & U256)
            return None

        # MUL
        if op == 0x02:
            a,_ = self._pop()
            b,_ = self._pop()
            self._push((a * b) & U256)
            return None

        # SUB
        if op == 0x03:
            a,_ = self._pop()
            b,_ = self._pop()
            self._push((a - b) & U256)
            return None

        # DIV
        if op == 0x04:
            a,_ = self._pop()
            b,_ = self._pop()
            self._push((a // b) & U256)
            return None

        # SDIV
        if op == 0x05:
            a,_ = self._pop()
            b,_ = self._pop()
            self._push((a // b) & U256)
            return None

        # MOD
        if op == 0x06:
            a,_ = self._pop()
            b,_ = self._pop()
            self._push((a % b) & U256)
            return None

        #SMOD
        if op == 0x07:
            a,_ = self._pop()
            b,_ = self._pop()
            self._push((a % b) & U256)
            return None

        # LT (unsigned)
        if op == 0x10:
            a,_ = self._pop()  # top of stack
            b,_ = self._pop()  # next
            self._push(1 if a < b else 0)
            return None

        # GT (unsigned)
        if op == 0x11:
            a,_ = self._pop()
            b,_ = self._pop()
            self._push(1 if a > b else 0)
            return None

        # SLT
        if op == 0x12:
            a,_ = self._pop()
            b,_ = self._pop()
            self._push(1 if (a < b) else 0)
            return None

        # SGT
        if op == 0x13:
            a,_ = self._pop()
            b,_ = self._pop()
            self._push(1 if (a > b) else 0)
            return None

        # EQ
        if op == 0x14:
            a,_ = self._pop()
            b,_ = self._pop()
            self._push(1 if a == b else 0)
            if a == self.sel:
                self.jumpfunction = True
            return None

        # ISZERO
        if op == 0x15:
            x,_ = self._pop()
            self._push(1 if x == 0 else 0)
            return None

        # AND
        if op == 0x16:
            a,_ = self._pop()
            b,_ = self._pop()
            self._push((a & b) & U256)
            return None

        # OR
        if op == 0x17:
            a,_ = self._pop()
            b,_ = self._pop()
            self._push((a | b) & U256)
            return None

        # XOR
        if op == 0x18:
            a,_ = self._pop()
            b,_ = self._pop()
            self._push((a ^ b) & U256)
            return None

        # NOT
        if op == 0x19:
            x,_ = self._pop()
            self._push(~x & U256)
            return None
        
        # BYTE
        if op == 0x1A:
            index,_ = self._pop()
            value,_ = self._pop()
            self._push(0) # fake it till you make it
            return None

        # SHL
        if op == 0x1B:
            shift,_ = self._pop()
            shift &= 0xFF
            x,_ = self._pop()
            self._push((x << shift) & U256)
            return None
        
        # SHR (EIP-145)
        if op == 0x1C:
            shift,_ = self._pop()
            shift &= 0xFF
            x,_ = self._pop()
            self._push((x >> shift) & U256)
            return None

        # KECCAK256
        if op == 0x20:
            self._pop()
            self._pop()
            self._push(0xceccac)
            return None

        # ADDRESS
        if op == 0x30:
            self._push(0xdeadfeed)
            return None

        # BALANCE
        if op == 0x31:
            self._pop()
            self._push(0) # every one is broke );
            return None

        # ORIGIN
        if op == 0x32:
            self._push(0xfee1dead)
            return None

        # CALLER
        if op == 0x33:
            self._push(0xdeadbeef)
            return None

        # CALLVALUE
        if op == 0x34:
            self._push(0)
            return None

        # CALLDATALOAD
        if op == 0x35:
            off,_ = self._pop()
            # Read 32 bytes from calldata starting at off
            w = 0
            for k in range(32):
                idx = off + k
                byte = self.calldata[idx] if 0 <= idx < len(self.calldata) else 0
                w = ((w << 8) | byte) & U256
            self._push(w)
            return None

        # CALLDATASIZE
        if op == 0x36:
            self._push(len(self.calldata))
            return None

        # CALLDATACOPY
        if op == 0x37:
            _ = self._pop()  # dest mem offset
            _ = self._pop()  # src calldata offset
            _ = self._pop()  # length
            return None

        # CODESIZE
        if op == 0x38:
            self._push(0xaffe) # dummy size
            return None

        # CODECOPY
        if op == 0x39:
            _ = self._pop()  # dest mem offset
            _ = self._pop()  # src code offset
            _ = self._pop()  # length
            return None

        # GASPRICE
        if op == 0x3A:
            self._push(0)
            return None

        # EXTCODESIZE
        if op == 0x3B:
            self._pop()
            self._push(0)
            return None

        # EXTCODECOPY
        if op == 0x3C:
            self._pop()  # addr
            self._pop()  # dest mem offset
            self._pop()  # src code offset
            self._pop()  # length
            return None

        # RETURNDATAIZE
        if op == 0x3D:
            self._push(0)
            return None

        # RETURNDATACOPY
        if op == 0x3E:
            self._pop()  # dest mem offset
            self._pop()  # src data offset
            self._pop()  # length
            return None

        # POP
        if op == 0x50:
            _ = self._pop()
            return None
        
        # MLOAD
        if op == 0x51:
            offset,_ = self._pop()
            if 0 <= offset < len(self.memory):
                value = int.from_bytes(self.memory[offset:offset+32], "big")
            else:
                value = 0
            self._push(value)
            return None

        # MSTORE
        if op == 0x52:
            # pop order matters: offset first, then value
            offset,_ = self._pop()
            value,_ = self._pop()
            if offset < 0:
                offset = 0
            end = offset + 32
            if end > len(self.memory):
                self.memory.extend(b"\x00" * (end - len(self.memory)))
            self.memory[offset:end] = (value & U256).to_bytes(32, "big")
            return None

        # MSTORE8
        if op == 0x53:
            _ = self._pop()
            _ = self._pop()
            return None

        # SLOAD
        if op == 0x54:
            self._pop()
            self._push(0x510ad) # notify we had a sload
            return None
        
        # SSTORE
        if op == 0x55:
            self._pop()
            self._pop()
            return None

        # JUMP
        if op == 0x56:
            dest,st_pc = self._pop()
            if 0 <= dest < len(self.code) and self.code[dest] == 0x5B:
                if self.recording:
                    self.pc -= 1
                    # hard jump always end a blob, we continue with recording the next blob
                    self._end_blob()
                    self._start_blob(dest)
                    self.labels.append((st_pc, dest))
                    
                    #print(f"JUMP to {dest:x} current stack: {self.dump_stack()} branches: {len(self.branches)}")
                self.pc = dest
                return None
            else:
                if len(self.branches) > 0:
                    # we have still branches, we might have taken a wrong turn with the wrong stack entries since we don't actually replay things
                    # destroy this blob and continue with the next branch. Explanation of this behavior in JUMPI handling.
                    if self.blobs[-1][1] == 0:
                        self.blobs.pop()
                    self.pc, self.stack = self._dequeue()
                    return None
                self.halted = True
                self.reason = "bad_jump"
                return None

        # JUMPI
        if op == 0x57:
            dest,st_pc = self._pop()
            cond,_ = self._pop()
            if cond != 0 or self.recording: # when recording blobs we want to follow every possible blob
                if 0 <= dest < len(self.code) and self.code[dest] == 0x5B:
                    if self.jumpfunction:
                        self.jumpfunction = False
                        self._start_blob(dest)
                        self.entry_pc = dest
                    elif self.recording:
                        # we are recording the blobs, we don't want to actually follow the jump
                        # so we just remember the branch and come back when this branch is done.
                        # we save the stack too.
                        # NOTE: we had a wild ride with JUMPI creating infinite new branches that we already did
                        # solution so far is to track the branches we worked on already and not reprocess them if we
                        # went through a few times (5 to be precise). Because of reuse of code segments we need to
                        # reprocess them, cause the stack might show a different jump address.
                        sig: StackSig = tuple(self.stack)
                        if sig not in self.worked_segments[dest]:
                            self.worked_segments[dest].add(sig)
                            self.branches.append((dest, self.stack.copy()))
                            self.labels.append((st_pc, dest))
                        #print(f"[[0x{self.pc - 1:x}]] JUMPI -> 0x{dest:x} branches: {self.branches}")
                        #print(f"created a branch: 0x{dest:x} -> {self.branches}")
                        return None
                    self.pc = dest
                    return None
                else:
                    if len(self.branches) > 1:
                        # we have still branches, we might have taken a wrong turn with the wrong stack entries since we don't actually replay things
                        # destroy this blob and continue with the next branch. This happens cause sometimes we have a JUMPI that will be invalid coming from our
                        # current stack state, but valid from another branch we will record later (maybe) or it will never be reached.
                        if self.blobs[-1][1] == 0:
                            self.blobs.pop()
                        self.pc, self.stack = self._dequeue()
                        return None
                    self.halted = True
                    self.reason = "bad_jumpi"
                    return None
            return None
        
        # GAS
        if op == 0x5A:
            self._push(0)
            return None

        # MCOPY
        if op == 0x5e:
            dest,_ = self._pop()
            src,_ = self._pop()
            size,_ = self._pop()
            self.memory[dest:dest+size] = self.memory[src:src+size]
            return None

        # CALL
        if op == 0xF1:
            self._pop()
            self._pop()
            self._pop()
            self._pop()
            self._pop()
            self._pop()
            self._pop()
            self._push(1)  # success

            return None
        
        # LOG calls
        if op in (0xA0, 0xA1, 0xA2, 0xA3, 0xA4):
            popcount = op - 0xA0 + 2
            for _ in range(popcount):
                self._pop()
            return None

        # (We can add more opcodes as needed)

        # Default: unknown opcode → halt (safe for POC)
        print(f"pc:{self.pc-1:x} op=0x{op:x}\nstack: {self.dump_stack()}")
        self.halted = True
        self.reason = f"unhandled_0x{op:02x}"
        return None
    
# ------------------------- Core Functionality ---------------------------------

def _adjust_offset(rh: str, kh: str, offset: int, table: dict[str, tuple[int, int, bytes, set[tuple[int,str,int]]]]) -> None:
    """
    Helper: find occurences of destinations in labels that point towards rh and replace it with kh
    and adjust offset to given offset
    """
    for h, (_, _, _, lset) in table.items():
        new_lset = set()
        for l in lset: #pc,h,off
            if l[1] == rh:
                new_lset.add((l[0], kh, offset))
                #print(f"adjusted label in blob {h} at pc: 0x{l[0]:x} from hash {rh} to {kh} with offset {offset}")
            else:
                new_lset.add(l)
        lset.clear()
        lset.update(new_lset)
    return

def blob_function(code: bytes, selector_hex: str, max_steps: int = 100000) -> tuple[list[tuple[int, int]],list[tuple[int, int]],int]:
    """
    Emulate from pc=0 with calldata having the given selector and return the
    first JUMPDEST PC we *land* on after a taken JUMP/JUMPI. None if not found.
    """
    sel = bytes.fromhex(selector_hex)
    cd = sel + b"\x00" * 64  # plenty of zeros
    vm = VM(code=code, calldata=cd, sel=int.from_bytes(sel, 'big'))

    steps = 0
    while not vm.halted and steps < max_steps:
        pc = vm.pc ## DEBUG PC
        dest = vm.step()
        #print(f"[VM] after step({steps}) at pc: 0x{pc:x} stack: {vm.dump_stack()}") ## DEBUG see whats going on
        if dest is not None:
            # We just landed on a JUMPDEST. Return it as the entry point.
            print(f"[VM] halted at pc: 0x{dest:x} - reason: {vm.reason}")
            return ([(0, 0)],[(0,0)],0)
        steps += 1

    if vm.halted and (vm.reason == "bad_jump" or vm.reason == "bad_jumpi"):
        print(f"[VM] halted - reason: {vm.reason}")
        exit(0)
    # print(f"labels: {vm.dump_labels()}")
    #print(f" remaining stack: {vm.dump_stack()}")
    return (vm.blobs, vm.labels, vm.entry_pc)

def find_functions(code: bytes, selectors: Iterable[str], table: dict[str, tuple[int, int, bytes, set[tuple[int,str,int]]]], func_starts: list[tuple[str,str]]) -> dict[str, tuple[int, int, bytes, set[tuple[int,str,int]]]]:
    """find function entry points and returns their blobs and labels in a table"""
    for sel in sorted(set(s.lower() for s in selectors)):
        #print(f"\nFinding blobs for selector: {sel}")
        blobs, labels, entry_pc = blob_function(code, sel)
        # there lies a possibility to run into an pseudo inifinite loop with JUMPI creating billions of branches.
        # to prevent this, the steps per functin search is limited to a 100000 steps, which is usually enough.
        # If the search is aborted due to step limit we will find an open blob at the end that needs to be removed.
        if blobs[-1][1] == 0:
            blobs.pop()
        #print(f"final blobs: {blobs}\nfinal labels: {labels}")
        unique_blobs = sorted(list(set(blobs)), key=lambda x: x[0])
        #print(f"existing unique blobs: {len(unique_blobs)}")
        #print(f"blobs: {unique_blobs}")
        
        # add all the unique blobs to the table only once! If a blob already exists from a previously
        # processed selector, we skip it.
        for start, end in unique_blobs:
            length = end - start
            bytes_blob = code[start:end+1]
            h = hashlib.sha256(bytes_blob).hexdigest()
            if h in table:
                #print(f"already have blob with hash: {h}, skipping sequence: {bytes_blob.hex()}")
                continue
            else:
                table[h] = (start, end, bytes_blob, set())
            #print(f"entered hash: {h}")
        
        # if table is empty we are at a good point to continue, but next selectors will append at the end
        # of the dict. so we need to sort the dict here before we continue
        table = dict(sorted(table.items(), key=lambda kv: kv[1][0]))

        # sort labels:
        sorted_labels = sorted(list(set(labels)), key=lambda x: x[0])
        #print(f"sorted labels: {sorted_labels}")

        # now we need to adjust the labels to point to (pc, hash, offset) according to the hashes
        # already inserted in the table and add the label positions to the blobs in the table.
        # rule of thumb is:
        # if the labels dest is the start of a blob, it points to the corresponding hash. all offsets
        # are for now 0. if the labels start is inside a blob, then we add that label to this blob.
        # NOTE: this will lead to potential duplicates of labels, since we could encouter overlaps from
        # blobs. we will need to handle this case later.
        hash_labels = []
        for start, dest in sorted_labels:
            #print(f"start: 0x{start:x} - dest: 0x{dest:x}")
            for h, (bstart,_,_,_) in table.items():
                if bstart == dest:
                    hash_labels.append((start, h))
            if hash_labels[-1][0] != start:
                print(f"WARNING: could not find blob start for label dest 0x{dest:x} at label pc 0x{start:x}")

        #print(f"\n\nhash_labels: {hash_labels}")
        #print(f"labels amounting to: {len(hash_labels)}")
        for item in hash_labels:
            for h, (bstart, bend, _ , lset) in table.items():
                if bstart <= item[0] and bend >= item[0]:
                    lset.add((item[0]-bstart,item[1],0))
        #for key, (bstart, bend, _, lset) in table.items():
        #    print(f"{key}||{bstart} - {bend} :: {lset}")

        # At this point, we need to remove the blobs that are included in bigger blobs. The only possible case
        # of a blob overlap is, that we have the same end of the blob but different entry points, just random
        # JUMPDEST in the code. When removing the blobs, we need to adjust any label, that points to the removed
        # blob and instead point to the larger blob and adjust the offset accordingly.
        rblst = []
        for kh, (bstart1, bend1, code1, lset1) in table.items():
            for rh, (bstart2, bend2, code2, lset2) in table.items():
                if bend1 > bend2:
                    # we found a blob that is before us
                    continue
                elif bend1 < bend2:
                    # there is no more blob that ends at the same point since the dict is ordered
                    break
                elif bstart1 < bstart2:
                    # found a larger blob that contains the current one we now need to find any label, that
                    # contains the to be removed blobs hash rhand replace it with the to keep blobs hash kh
                    # and adjust the offset
                    if kh in rblst:
                        # this blob has already been removed and should not replace another blob. there is another
                        # blob incoming, that contains this blob already.
                        continue
                    _adjust_offset(rh, kh, bstart2 - bstart1, table)
                    rblst.append(rh)
                    break
                else:
                    # we found ourselves so we just continue the seek
                    continue

        # now we can remove all blobs from the table that need to be removed
        for rh in rblst:
            del table[rh]

        for h, (start, _, _, _) in table.items():
            if start == entry_pc:
                func_starts.append((sel, h))
                break

    return table
