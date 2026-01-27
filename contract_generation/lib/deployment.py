#!/usr/bin/env python3

from pathlib import Path

def create_deployment_code(runtimeLength: int) -> bytearray :
    """
    Create deployment code with correct runtime and overall length
    """
    # TODO at some point the name of output should be hardcoded like runtime.bin and deployment.bin
    dpl_file = Path("./build/high/deployment.bin").read_text(encoding="utf-8", errors="ignore").strip()
    rntm_file = Path("./build/high/runtime.bin").read_text(encoding="utf-8", errors="ignore").strip()

    # the deployment code will stay the same but have some minor adjustments. we need to change length fields
    # reagarding the total size of the bytecode (which is used to find the arguments behind the code to deploy)
    # and the size of the runtime code (which is used to copy the runtime code into memory)
    # total size of bytecode is used usually twice in the beginning to calculate the size of the ctor arguments
    # runtime size is used once to copy the runtime code into memory at the end of deployment code before CODECOPY
    
    # Calculate runtime size of bytecode
    rntm_size = len(rntm_file)//2
    hex_size = format(rntm_size, '04x')
    pattern_sizerntm = f"61{hex_size}"
    
    # Calculate size of all code
    all_size = len(dpl_file)//2
    hex_size = format(all_size, '04x')
    pattern_sizeall = f"61{hex_size}"

    # Calculate size of deployment code for calculation of new total size
    cutoff = dpl_file[0:-rntm_size*2]
    dpl_size = len(cutoff)//2

    newrntm_hex = format(runtimeLength, '04x')
    pattern_newrntm = f"61{newrntm_hex}"
    newtotal = dpl_size + runtimeLength
    newtotal_hex = format(newtotal, '04x')
    pattern_newtotal = f"61{newtotal_hex}"

    final_dpl = cutoff.replace(pattern_sizeall, pattern_newtotal).replace(pattern_sizerntm, pattern_newrntm)
    
    return bytearray.fromhex(final_dpl)

# if debugging is needed, just use the main
if __name__ == "__main__":
    dpl_file = Path("./build/high/deployment.bin").read_text(encoding="utf-8", errors="ignore").strip()
    rntm_file = Path("./build/high/runtime.bin").read_text(encoding="utf-8", errors="ignore").strip()

    # the deployment code will stay the same but have some minor adjustments. we need to change length fields
    # reagarding the total size of the bytecode (which is used to find the arguments behind the code to deploy)
    # and the size of the runtime code (which is used to copy the runtime code into memory)
    # total size of bytecode is used usually twice in the beginning to calculate the size of the ctor arguments
    # runtime size is used once to copy the runtime code into memory at the end of deployment code before CODECOPY
    
    # Calculate runtime size of bytecode
    rntm_size = len(rntm_file)//2
    hex_size = format(rntm_size, '04x')
    pattern_sizerntm = f"61{hex_size}"
    
    # Calculate size of all code
    all_size = len(dpl_file)//2
    hex_size = format(all_size, '04x')
    pattern_sizeall = f"61{hex_size}"

    # Calculate size of deployment code for calculation of new total size
    cutoff = dpl_file[0:-rntm_size*2]
    dpl_size = len(cutoff)//2


    print(f"patterns to find:\nRuntime length:{pattern_sizerntm}\nAll code length:{pattern_sizeall}")

    # new runtime size should be passed
    newrntm = 2600
    newrntm_hex = format(newrntm, '04x')
    pattern_newrntm = f"61{newrntm_hex}"
    newtotal = dpl_size + newrntm
    newtotal_hex = format(newtotal, '04x')
    pattern_newtotal = f"61{newtotal_hex}"
    print(f"New runtime length pattern to insert: {pattern_newrntm}")
    print(f"New total length pattern to insert: {pattern_newtotal}")
    print(f"new total size: {newtotal} dpl_size: {dpl_size:x} newrntm: {newrntm}")

    #print(f"{dpl_file}\n ")
    final_dpl = cutoff.replace(pattern_sizeall, pattern_newtotal).replace(pattern_sizerntm, pattern_newrntm)
    print(f"Final deployment code:\n{final_dpl}")
    exit()