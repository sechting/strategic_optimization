// nft.test.mjs
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { ethers } from "ethers";

const __filename = fileURLToPath(import.meta.url);
const __dirname  = path.dirname(__filename);

// --- TeX helpers -----------------------------------------------------------
function texFilePath() {
  return path.join(__dirname, "..", "poc_nftresults.tex");
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


const CTOR_HEX = ""; // no constructor args for MyNFT

const FILES = [
  ["HIGH",  path.join(__dirname, "..", "build", "high",  "deployment.bin")],
  ["LOW",   path.join(__dirname, "..", "build", "low",   "deployment.bin")],
  ["FINAL", path.join(__dirname, "..", "build", "final", "deployment.bin")],
];

// Minimal ABI for the tests we do now.
// We can extend this later as we add verifySymbol / verifyMint etc.
const ERC721_ABI = [
  "function name() view returns (string)",
  "function symbol() view returns (string)",
  "function safeMint(address to)",
  "function balanceOf(address owner) view returns (uint256)",
  "function ownerOf(uint256 tokenId) view returns (address)",
  "function tokenURI(uint256 tokenId) view returns (string)",
  "function transferFrom(address from, address to, uint256 tokenId)",
  "function safeTransferFrom(address from, address to, uint256 tokenId)",
  "function approve(address to, uint256 tokenId)",
  "function getApproved(uint256 tokenId) view returns (address)",
  "function safeTransferFrom(address from, address to, uint256 tokenId)",
  "function safeTransferFrom(address from, address to, uint256 tokenId, bytes data)",
];

// ---- helpers --------------------------------------------------------------

// Deploy one variant using explicit nonce management
async function deployVariant(label, filePath, deployer, nonce, ctorHex = "") {
  let raw = fs.readFileSync(filePath, "utf8").trim();
  if (!raw) {
    throw new Error(`Empty deployment.bin for ${label} at ${filePath}`);
  }

  let data = "0x" + raw;
  if (ctorHex) {
    const suffix = ctorHex.startsWith("0x") ? ctorHex.slice(2) : ctorHex;
    data = data + suffix;
  }

  console.log(`\n[${label}] sending deployment tx with nonce ${nonce}...`);
  const tx = await deployer.sendTransaction({ data, nonce });
  const receipt = await tx.wait();

  const addr = receipt.contractAddress;
  const gas  = receipt.gasUsed.toString();

  console.log(`[${label}] deployed at ${addr} (gas: ${gas})`);
  return { address: addr, gasUsed: gas };
}

// Ensure there is code at the deployed address
async function verifyCodeExists(provider, address, label) {
  const code = await provider.getCode(address);
  if (!code || code === "0x") {
    throw new Error(`[${label}] No code at deployed address ${address}`);
  }
  console.log(
    `[${label}] code size: ${code.length / 2 - 1} bytes (non-zero OK)`
  );
}

// Test name() == "MyNFT"
async function verifyName(provider, address, label) {
  const nft = new ethers.Contract(address, ERC721_ABI, provider);

  const name = await nft.name();
  console.log(`[${label}] name() => "${name}"`);

  const expected = "MyNFT";
  if (name !== expected) {
    throw new Error(
      `[${label}] name() mismatch: expected "${expected}", got "${name}"`
    );
  }
}

// Test symbol() == "MNFT"
async function verifySymbol(provider, address, label) {
  const nft = new ethers.Contract(address, ERC721_ABI, provider);

  const symbol = await nft.symbol();
  console.log(`[${label}] symbol() => "${symbol}"`);

  const expected = "MNFT";
  if (symbol !== expected) {
    throw new Error(
      `[${label}] symbol() mismatch: expected "${expected}", got "${symbol}"`
    );
  }
}

async function verifyMintAndOwnership(signer, address, label, nonce) {
  const nft = new ethers.Contract(address, ERC721_ABI, signer);
  const ownerAddr = signer.address;

  // balance before mint
  const beforeBal = await nft.balanceOf(ownerAddr);
  console.log(`[${label}] balanceOf(deployer) before mint = ${beforeBal.toString()}`);

  // mint 1 token to deployer using explicit nonce
  const tx = await nft.safeMint(ownerAddr, { nonce });
  const receipt = await tx.wait();
  console.log(`[${label}] safeMint tx gas used = ${receipt.gasUsed.toString()}`);
  nonce += 1;

  // balance after mint
  const afterBal = await nft.balanceOf(ownerAddr);
  console.log(`[${label}] balanceOf(deployer) after mint = ${afterBal.toString()}`);

  if (afterBal !== beforeBal + 1n) {
    throw new Error(
      `[${label}] balanceOf mismatch after mint: expected ${beforeBal + 1n}, got ${afterBal}`
    );
  }

  // since _nextTokenId starts at 1, first mint should be tokenId 1
  const owner1 = await nft.ownerOf(1n);
  console.log(`[${label}] ownerOf(1) = ${owner1}`);

  if (owner1.toLowerCase() !== ownerAddr.toLowerCase()) {
    throw new Error(
      `[${label}] ownerOf(1) mismatch: expected ${ownerAddr}, got ${owner1}`
    );
  }

  // optional: check tokenURI(1), just log it
  try {
    const uri1 = await nft.tokenURI(1n);
    console.log(`[${label}] tokenURI(1) = "${uri1}"`);
  } catch (e) {
    console.warn(`[${label}] tokenURI(1) reverted: ${e.message}`);
  }

  return {mintNonce: nonce, gasUsed: receipt.gasUsed};
}

// nft: ethers.Contract
// deployer, recipient: ethers.Wallet (or Signer)
// label: string ("HIGH"/"LOW"/"FINAL")
// nonce: bigint (manual nonce for deployer)
async function verifySimpleTransfer(nft, deployer, recipient, label, nonce) {
  const tokenId = 1n; // assuming already minted to deployer

  const from = await deployer.getAddress();
  const to   = await recipient.getAddress();

  const ownerBefore    = await nft.ownerOf(tokenId);
  const balFromBefore  = await nft.balanceOf(from);
  const balToBefore    = await nft.balanceOf(to);

  console.log(`[${label}] owner before transfer = ${ownerBefore}`);
  console.log(
    `[${label}] balances before transfer: from=${balFromBefore} to=${balToBefore}`
  );

  const nftFromDeployer = nft.connect(deployer);

  // use the manual nonce here
  const tx = await nftFromDeployer.transferFrom(from, to, tokenId, {
    nonce,
    // you *can* also force gasLimit here if you want:
    // gasLimit: 500000n,
  });
  nonce += 1;

  const receipt = await tx.wait();
  console.log(
    `[${label}] transferFrom gasUsed = ${receipt.gasUsed.toString()}`
  );

  const ownerAfter    = await nft.ownerOf(tokenId);
  const balFromAfter  = await nft.balanceOf(from);
  const balToAfter    = await nft.balanceOf(to);

  console.log(`[${label}] owner after transfer = ${ownerAfter}`);
  console.log(
    `[${label}] balances after transfer: from=${balFromAfter} to=${balToAfter}`
  );

  if (ownerAfter.toLowerCase() !== to.toLowerCase()) {
    throw new Error(`[${label}] transferFrom failed: wrong owner after`);
  }

  if (balFromAfter !== balFromBefore - 1n || balToAfter !== balToBefore + 1n) {
    throw new Error(
      `[${label}] transferFrom failed: balances not updated correctly`
    );
  }

  // return the next nonce for deployer
  return { transferNonce: nonce, gasUsed: receipt.gasUsed };
}

// nft: ethers.Contract
// owner: signer that currently owns tokenId (deployer)
// spender: signer that will be approved (second)
// label: "HIGH" | "LOW" | "FINAL"
// nonce: bigint (current nonce for owner)
async function verifyApprove(nft, owner, spender, tokenId, label, nonce) {
  const ownerAddr   = await owner.getAddress();
  const spenderAddr = await spender.getAddress();

  console.log(
    `[${label}] calling approve(spender=${spenderAddr}, tokenId=${tokenId}) from ${ownerAddr}`
  );

  const nftFromOwner = nft.connect(owner);

  const tx = await nftFromOwner.approve(spenderAddr, tokenId, {
    nonce,
    // optionally: gasLimit: 500000n,
  });
  nonce += 1;

  const receipt = await tx.wait();
  const gasUsed = receipt.gasUsed;

  console.log(
    `[${label}] approve gasUsed = ${gasUsed.toString()}`
  );

  // verify on-chain state
  const approved = await nft.getApproved(tokenId);
  console.log(`[${label}] getApproved(${tokenId}) = ${approved}`);

  if (approved.toLowerCase() !== spenderAddr.toLowerCase()) {
    throw new Error(
      `[${label}] approve failed: getApproved != spender (got ${approved}, expected ${spenderAddr})`
    );
  }

  // return both next nonce + gasUsed so caller can record it
  return { approveNonce: nonce, gasUsed: gasUsed };
}

// nft: ethers.Contract
// fromSigner: current  approved spender (e.g. deployer)
// toSigner: recipient signer (EOA) for convenience in tests
// tokenId: bigint
// label: "HIGH" | "LOW" | "FINAL"
// nonce: bigint (for fromSigner)
// code might be a little messy, but it works for now, sending the approved nft back to deployer
async function verifySafeTransfer(
  nft,
  fromSigner,
  toSigner,
  tokenId,
  label,
  nonce
) {
  const from = await fromSigner.getAddress();
  const to   = await toSigner.getAddress();

  const ownerBefore   = await nft.ownerOf(tokenId);
  const balFromBefore = await nft.balanceOf(from);
  const balToBefore   = await nft.balanceOf(to);

  console.log(`[${label}] safeTransferFrom owner before = ${ownerBefore}`);
  console.log(
    `[${label}] balances before: from=${balFromBefore} to=${balToBefore}`
  );

  const nftFromSender = nft.connect(fromSigner);

  // non-empty data so we exercise the 4-arg variant
  const data = "0x1234";

  // disambiguate overload: (address,address,uint256,bytes)
  const safeTransferWithData =
    nftFromSender["safeTransferFrom(address,address,uint256,bytes)"];

  const tx = await safeTransferWithData(to, from, tokenId, data, {
    nonce,
    // gasLimit: 500000n, // optional
  });
  nonce += 1;

  const receipt = await tx.wait();
  const gasUsed = receipt.gasUsed;

  console.log(
    `[${label}] safeTransferFrom gasUsed = ${gasUsed.toString()}`
  );

  const ownerAfter   = await nft.ownerOf(tokenId);
  const balFromAfter = await nft.balanceOf(from);
  const balToAfter   = await nft.balanceOf(to);

  console.log(`[${label}] safeTransferFrom owner after = ${ownerAfter}`);
  console.log(
    `[${label}] balances after: from=${balFromAfter} to=${balToAfter}`
  );

  if (ownerAfter.toLowerCase() !== from.toLowerCase()) {
    throw new Error(`[${label}] safeTransferFrom failed: wrong owner after`);
  }

  if (balFromAfter !== balFromBefore + 1n || balToAfter !== balToBefore - 1n) {
    throw new Error(
      `[${label}] safeTransferFrom failed: balances not updated correctly`
    );
  }

  return { safeTransferNonce: nonce, gasUsed: gasUsed };
}

// ---- main -----------------------------------------------------------------

async function main() {
  const provider = new ethers.JsonRpcProvider(RPC_URL);
  const deployer = new ethers.Wallet(DEPLOYER_PK, provider);
  const second   = new ethers.Wallet(SECOND_PK, provider);

  const gasBook = {deploy: {}, mint:{},transferFrom:{}, approve:{},safeTransferFrom:{}};
  console.log("Deployer:", deployer.address);

  let nonce = await provider.getTransactionCount(deployer.address);
  let secondNonce = await provider.getTransactionCount(second.address);
  console.log("Starting nonce:", nonce);

  for (const [label, filePath] of FILES) {
    // 1) Deploy
    const { address, gasUsed: deployGas } = await deployVariant(label, filePath, deployer, nonce, CTOR_HEX);
    gasBook.deploy[label.toLowerCase()] = deployGas;
    const nft = new ethers.Contract(address, ERC721_ABI, provider);
    nonce += 1;

    // 2) Generic sanity check
    await verifyCodeExists(provider, address, label);

    // 3) Specific tests
    await verifyName(provider, address, label);
    await verifySymbol(provider, address, label);
    const {mintNonce, gasUsed: mintGas} = await verifyMintAndOwnership(deployer, address, label, nonce);
    nonce = mintNonce;
    gasBook.mint[label.toLowerCase()] = mintGas;
    const {transferNonce, gasUsed: transferGas} = await verifySimpleTransfer(nft, deployer, second, label, nonce);
    nonce = transferNonce;
    gasBook.transferFrom[label.toLowerCase()] = transferGas;
    const {nextNonce: approveNonce, gasUsed: approveGas} = await verifyApprove(nft, second, deployer, 1n, label, secondNonce);
    secondNonce = approveNonce;
    gasBook.approve[label.toLowerCase()] = approveGas;
    const {safeTransferNonce, gasUsed: safeTransferGas} = await verifySafeTransfer(nft, deployer, second, 1n, label, nonce);
    nonce = safeTransferNonce;
    gasBook.safeTransferFrom[label.toLowerCase()] = safeTransferGas;
  }
    console.log("\nFunction gas summary (success cases only):");
  const gasRows = [
    { function: "deployment",         high: gasBook.deploy.high || "-", low: gasBook.deploy.low || "-", final: gasBook.deploy.final || "-" },
    { function: "mint",         high: gasBook.mint.high        || "-", low: gasBook.mint.low        || "-", final: gasBook.mint.final        || "-" },
    { function: "approve",      high: gasBook.approve.high  || "-", low: gasBook.approve.low  || "-", final: gasBook.approve.final  || "-" },
    { function: "transferFrom",     high: gasBook.transferFrom.high || "-", low: gasBook.transferFrom.low || "-", final: gasBook.transferFrom.final || "-" },
    { function: "safeTransferFrom", high: gasBook.safeTransferFrom.high || "-", low: gasBook.safeTransferFrom.low || "-", final: gasBook.safeTransferFrom.final || "-" },
  ];
  console.table(gasRows);

  // ===== Write LaTeX macro file =====
  const tex =
    "% Auto-generated by test/nft.test.mjs — Do not edit by hand\n" +
    "% Gas units are raw transaction gasUsed\n" +
    texCmd("NFTdeploymentHigh", safeVal(gasBook.deploy.high)) +
    texCmd("NFTdeploymentLow",  safeVal(gasBook.deploy.low)) +
    texCmd("NFTdeploymentFinal",safeVal(gasBook.deploy.final)) +
    // mint
    texCmd("NFTmintHigh",       safeVal(gasBook.mint.high)) +
    texCmd("NFTmintLow",        safeVal(gasBook.mint.low)) +
    texCmd("NFTmintFinal",      safeVal(gasBook.mint.final)) +
    // approve
    texCmd("NFTapproveHigh",     safeVal(gasBook.approve.high)) +
    texCmd("NFTapproveLow",      safeVal(gasBook.approve.low)) +
    texCmd("NFTapproveFinal",    safeVal(gasBook.approve.final)) +
    // transferFrom
    texCmd("NFTtransferFromHigh",    safeVal(gasBook.transferFrom.high)) +
    texCmd("NFTtransferFromLow",     safeVal(gasBook.transferFrom.low)) +
    texCmd("NFTtransferFromFinal",   safeVal(gasBook.transferFrom.final)) +
    // safeTransferFrom
    texCmd("NFTsafeTransferFromHigh",  safeVal(gasBook.safeTransferFrom.high)) +
    texCmd("NFTsafeTransferFromLow",   safeVal(gasBook.safeTransferFrom.low)) +
    texCmd("NFTsafeTransferFromFinal", safeVal(gasBook.safeTransferFrom.final))

  const outPath = texFilePath();
  fs.writeFileSync(outPath, tex);
  console.log(`\nWrote LaTeX macros → ${outPath}`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
