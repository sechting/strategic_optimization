// test/token.test.mjs
// Deploy HIGH, LOW, FINAL with ctor args ("Test","TST") sequentially.
// Measure gas for approve/transfer/transferFrom (success paths only).
// Also call all ERC20 view/optional functions and summarize their return values.

import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { ethers } from "ethers";

const __filename = fileURLToPath(import.meta.url);
const __dirname  = path.dirname(__filename);

// --- TeX helpers -----------------------------------------------------------
function texFilePath() {
  return path.join(__dirname, "..", "poc_results.tex");
}
function texCmd(name, value) {
  return `\\newcommand{\\${name}}{${value}}\n`;
}
function safeVal(v) { return (v && v !== "-") ? String(v) : "NA"; }


// ---- config ---------------------------------------------------------------
const RPC_URL = "http://127.0.0.1:8545";
const DEPLOYER_PK =
  "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80";
const SECOND_PK =
  "0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d";

// ABI-encoded ctor args for ("Test","TST") => 192 bytes
const CTOR_HEX =
  "0x" +
  "0000000000000000000000000000000000000000000000000000000000000040" +
  "0000000000000000000000000000000000000000000000000000000000000080" +
  "0000000000000000000000000000000000000000000000000000000000000004" +
  "5465737400000000000000000000000000000000000000000000000000000000" +
  "0000000000000000000000000000000000000000000000000000000000000003" +
  "5453540000000000000000000000000000000000000000000000000000000000";

// Creation bytecode files (NO 0x in files)
const FILES = [
  ["HIGH",  path.join(__dirname, "..", "build", "high",  "deployment.bin")],
  ["LOW",   path.join(__dirname, "..", "build", "low",   "deployment.bin")],
  ["FINAL", path.join(__dirname, "..", "build", "final", "deployment.bin")],
];

// Minimal ERC20 ABI (full standard + optional views)
const ERC20_ABI = [
  "function name() view returns (string)",
  "function symbol() view returns (string)",
  "function decimals() view returns (uint8)",
  "function totalSupply() view returns (uint256)",
  "function balanceOf(address) view returns (uint256)",
  "function allowance(address,address) view returns (uint256)",
  "function approve(address,uint256) returns (bool)",
  "function transfer(address,uint256) returns (bool)",
  "function transferFrom(address,address,uint256) returns (bool)",
];

// ---- helpers --------------------------------------------------------------
function readHexFile0x(p) {
  const hex = fs.readFileSync(p, "utf8").trim().replace(/^0x/i, "").replace(/\s+/g, "");
  if (!hex) throw new Error(`empty bytecode at ${p}`);
  if (!/^([0-9a-fA-F]{2})+$/.test(hex)) throw new Error(`non-hex content in ${p}`);
  return ("0x" + hex).toLowerCase();
}
function hexLen(hex) { return (hex.length - 2) / 2; }
function summarize(label, hex) {
  console.log(`${label}: ${hexLen(hex)} bytes | head ${hex.slice(2, 66)} | tail ${hex.slice(-64)}`);
}

async function getFees(provider) {
  const f = await provider.getFeeData();
  const maxPriorityFeePerGas = f.maxPriorityFeePerGas ?? 1n * 10n ** 9n;
  const maxFeePerGas = f.maxFeePerGas ?? (maxPriorityFeePerGas + 2n * 10n ** 9n);
  return { maxFeePerGas, maxPriorityFeePerGas };
}

// explicit nonces per signer
const NONCES = new Map();
async function nextNonce(provider, addr) {
  if (!NONCES.has(addr)) {
    NONCES.set(addr, await provider.getTransactionCount(addr, "pending"));
  }
  const n = NONCES.get(addr);
  NONCES.set(addr, n + 1);
  return n;
}

async function sendTx(txFn, { signer, fees, gasFallback = 200_000n }) {
  let gasLimit = null;
  try { gasLimit = await txFn.estimateGas(); }
  catch { gasLimit = gasFallback; }
  const addr = await signer.getAddress();
  const nonce = await nextNonce(signer.provider, addr);
  const tx = await txFn({
    gasLimit,
    maxFeePerGas: fees.maxFeePerGas,
    maxPriorityFeePerGas: fees.maxPriorityFeePerGas,
    nonce,
    type: 2,
  });
  const rc = await tx.wait();
  return { tx, rc };
}

async function viewOr(tag, fn) {
  try { return await fn(); } catch { return "<revert>"; }
}

// ---- remove intrinsic gas cost --------------------------------------------
function calcIntrinsicGas(data) {
  // data is 0x-prefixed hex string
  const hex = data.startsWith("0x") ? data.slice(2) : data;
  const byteLen = hex.length / 2;

  let zeros = 0;
  let nonzeros = 0;

  for (let i = 0; i < byteLen; i++) {
    const byte = hex.slice(i * 2, i * 2 + 2);
    if (byte === "00") {
      zeros++;
    } else {
      nonzeros++;
    }
  }

  const base = 21_000n;
  return base + 4n * BigInt(zeros) + 16n * BigInt(nonzeros);
}

// ---- main -----------------------------------------------------------------
async function main() {
  const provider = new ethers.JsonRpcProvider(RPC_URL);
  const deployer = new ethers.Wallet(DEPLOYER_PK, provider);
  const acct1    = new ethers.Wallet(SECOND_PK, provider);

  NONCES.clear();
  NONCES.set(deployer.address, await provider.getTransactionCount(deployer.address, "pending"));
  NONCES.set(acct1.address,    await provider.getTransactionCount(acct1.address, "pending"));

  const block = await provider.getBlock("latest");
  const blockCap = block.gasLimit ?? 30_000_000n;
  const GAS_LIMIT = blockCap - 100_000n;
  const fees = await getFees(provider);

  console.log(`RPC: ${RPC_URL}`);
  console.log(`Deployer: ${deployer.address}`);
  console.log(`Acct1:    ${acct1.address}`);
  console.log(`Block gas limit: ${blockCap.toString()} → tx gasLimit: ${GAS_LIMIT.toString()}\n`);

  // Gas summary for mutating calls
  const gasBook = { approve:{}, transfer:{}, transferFrom:{}, balanceOf:{} };
  // View results summary
  const viewRows = [];

  const deployRows = [];

  for (const [label, file] of FILES) {
    console.log(`=== ${label} ===`);
    if (!fs.existsSync(file)) { console.log(`  skip: file missing → ${file}\n`); continue; }

    const creation = readHexFile0x(file);
    summarize("  creation", creation);
    const init = (creation + CTOR_HEX.slice(2)).toLowerCase();
    console.log(`  bytes: creation=${hexLen(creation)}, ctor=${hexLen(CTOR_HEX)}, init=${hexLen(init)}`);

    // Deploy (explicit nonce)
    const depNonce = await nextNonce(provider, deployer.address);
    const tx = await deployer.sendTransaction({
      to: undefined,
      data: init,
      value: 0n,
      type: 2,
      gasLimit: GAS_LIMIT,
      maxFeePerGas: fees.maxFeePerGas,
      maxPriorityFeePerGas: fees.maxPriorityFeePerGas,
      nonce: depNonce,
    });
    const rc = await tx.wait();
    const addr = rc.contractAddress;
    console.log(`  tx: ${tx.hash}`);
    console.log(`  creation gasUsed: ${rc.gasUsed.toString()}`);
    console.log(`  contractAddress: ${addr}`);

    deployRows.push({ variant: label, address: addr, deployGas: rc.gasUsed.toString() });

    
    const token = new ethers.Contract(addr, ERC20_ABI, provider);

    const tname = await viewOr("name", () => token.name());

    // ---- views (optional & standard) ----
    const name      = await viewOr("name",      () => token.name());
    const symbol    = await viewOr("symbol",    () => token.symbol());
    const decimals  = await viewOr("decimals",  () => token.decimals());
    const total     = await viewOr("totalSupply", () => token.totalSupply().then(x=>x.toString()));
    const balDep0   = await viewOr("balanceOf", () => token.balanceOf(deployer.address).then(x=>x.toString()));
    const balAcc0   = await viewOr("balanceOf", () => token.balanceOf(acct1.address).then(x=>x.toString()));
    const allow0    = await viewOr("allowance", () => token.allowance(deployer.address, acct1.address).then(x=>x.toString()));

    console.log(`  initial balance deployer: ` + balDep0);
    // record a row for these views (no gas — eth_call)
    viewRows.push({
      variant: label, address: addr,
      name: String(name), symbol: String(symbol), decimals: String(decimals),
      totalSupply: String(total), balanceDeployer: String(balDep0), balanceAcct1: String(balAcc0),
      allowanceDepToAcc: String(allow0),
    });

    // ---- success-path mutations ----
    // approve: deployer -> acct1
    try {
      const { tx: atx, rc: arc } = await sendTx(
        token.connect(deployer).approve.bind(token.connect(deployer), acct1.address, 10n),
        { signer: deployer, fees, gasFallback: 120_000n }
      );
      const intrinsicGas = calcIntrinsicGas(token.interface.encodeFunctionData("approve", [acct1.address, 10n]));
      console.log(`  approve ok | hash=${atx.hash} | gasUsed=${(arc.gasUsed-intrinsicGas).toString()}`);
      gasBook.approve[label.toLowerCase()] = (arc.gasUsed - intrinsicGas).toString();
    } catch (e) {
      console.log(`  approve unexpected revert:`, e?.shortMessage || e?.message || e);
    }

    // transfer: deployer -> acct1 (1 token)
    try {
      const { tx: ttx, rc: trc } = await sendTx(
        token.connect(deployer).transfer.bind(token.connect(deployer), acct1.address, 1n),
        { signer: deployer, fees, gasFallback: 120_000n }
      );
      const intrinsicGas = calcIntrinsicGas(token.interface.encodeFunctionData("transfer", [acct1.address, 1n]));
      console.log(`  transfer ok | hash=${ttx.hash} | gasUsed=${(trc.gasUsed-intrinsicGas).toString()}`);
      gasBook.transfer[label.toLowerCase()] = (trc.gasUsed - intrinsicGas).toString();
    } catch (e) {
      console.log(`  transfer unexpected revert:`, e?.shortMessage || e?.message || e);
    }

    // transferFrom: acct1 spends deployer's tokens (requires allowance above)
    try {
      const { tx: ftx, rc: frc } = await sendTx(
        token.connect(acct1).transferFrom.bind(token.connect(acct1), deployer.address, acct1.address, 1n),
        { signer: acct1, fees, gasFallback: 150_000n }
      );
      const intrinsicGas = calcIntrinsicGas(token.interface.encodeFunctionData("transferFrom", [deployer.address, acct1.address, 1n]));
      console.log(`  transferFrom ok | hash=${ftx.hash} | gasUsed=${(frc.gasUsed-intrinsicGas).toString()}`);
      gasBook.transferFrom[label.toLowerCase()] = (frc.gasUsed - intrinsicGas).toString();
    } catch (e) {
      console.log(`  transferFrom unexpected revert:`, e?.shortMessage || e?.message || e);
    }

    // balanceOf: deployer after tansfers
    const gasEstimate = await token.connect(deployer).balanceOf.estimateGas(deployer.address);
    const intrinsicGas = calcIntrinsicGas(token.interface.encodeFunctionData("balanceOf", [deployer.address]));
    console.log(`balanceOf gas estimate: ${(gasEstimate - intrinsicGas).toString()}`);
    gasBook.balanceOf[label.toLowerCase()] = (gasEstimate - intrinsicGas).toString();

    
    // final balances (informational)
    const balDep1 = await token.balanceOf(deployer.address).then(x=>x.toString());
    const balAcc1 = await token.balanceOf(acct1.address).then(x=>x.toString());
    const allow1  = await token.allowance(deployer.address, acct1.address).then(x=>x.toString());
    console.log(`  balances/allowance (after): deployer=${balDep1} acct1=${balAcc1} allowance=${allow1}\n`);
  }

  console.log("Deploy summary:");
  console.table(deployRows);

  console.log("\nFunction gas summary (success cases only):");
  const gasRows = [
    { function: "approve",      high: gasBook.approve.high  || "-", low: gasBook.approve.low  || "-", final: gasBook.approve.final  || "-" },
    { function: "transfer",     high: gasBook.transfer.high || "-", low: gasBook.transfer.low || "-", final: gasBook.transfer.final || "-" },
    { function: "transferFrom", high: gasBook.transferFrom.high || "-", low: gasBook.transferFrom.low || "-", final: gasBook.transferFrom.final || "-" },
    { function: "balanceOf",   high: gasBook.balanceOf.high || "-", low: gasBook.balanceOf.low || "-", final: gasBook.balanceOf.final || "-" },
  ];
  console.table(gasRows);

  console.log("\nERC20 views summary (return values):");
  console.table(viewRows);
    // ===== Write LaTeX macro file =====
  const depGas = { high: "NA", low: "NA", final: "NA" };
  for (const r of deployRows) depGas[r.variant.toLowerCase()] = r.deployGas;

  const tex =
    "% Auto-generated by test/token.test.mjs — Do not edit by hand\n" +
    "% Gas units are raw transaction gasUsed\n" +
    texCmd("deploymentHigh", safeVal(depGas.high)) +
    texCmd("deploymentLow",  safeVal(depGas.low)) +
    texCmd("deploymentFinal",safeVal(depGas.final)) +
    // approve
    texCmd("approveHigh",     safeVal(gasBook.approve.high)) +
    texCmd("approveLow",      safeVal(gasBook.approve.low)) +
    texCmd("approveFinal",    safeVal(gasBook.approve.final)) +
    // transfer
    texCmd("transferHigh",    safeVal(gasBook.transfer.high)) +
    texCmd("transferLow",     safeVal(gasBook.transfer.low)) +
    texCmd("transferFinal",   safeVal(gasBook.transfer.final)) +
    // transferFrom
    texCmd("transferFromHigh",  safeVal(gasBook.transferFrom.high)) +
    texCmd("transferFromLow",   safeVal(gasBook.transferFrom.low)) +
    texCmd("transferFromFinal", safeVal(gasBook.transferFrom.final)) +
    // balanceOf
    texCmd("balanceOfHigh",    safeVal(gasBook.balanceOf.high)) +
    texCmd("balanceOfLow",     safeVal(gasBook.balanceOf.low)) +
    texCmd("balanceOfFinal",   safeVal(gasBook.balanceOf.final));

  const outPath = texFilePath();
  fs.writeFileSync(outPath, tex);
  console.log(`\nWrote LaTeX macros → ${outPath}`);

}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
