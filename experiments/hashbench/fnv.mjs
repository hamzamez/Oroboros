// Hand-written FNV-1a for JavaScript (hashbench-2026-10-27): node fnv.mjs MODE FILE
//   MODE pair:   64 bits as two 32-bit halves, the split pair u64repr-2026-09-22 measured
//   MODE bigint: 64 bits as a BigInt reduced each step, as many write it
import { readFileSync } from "node:fs";
const [mode, file] = process.argv.slice(2);
const b = readFileSync(file);
let h32 = 0x811c9dc5;
for (let i = 0; i < b.length; i++) h32 = Math.imul(h32 ^ b[i], 16777619) >>> 0;
let out64;
if (mode === "pair") {
  // h·(2^40 + 435) mod 2^64: the low half times 435, its carry, and the low
  // half's bits that 2^40 moves into the high half
  let hi = 0xcbf29ce4, lo = 0x84222325;
  for (let i = 0; i < b.length; i++) {
    const x = (lo ^ b[i]) >>> 0;
    const t = x * 435;
    lo = t >>> 0;
    hi = (Math.imul(hi, 435) + Math.floor(t / 4294967296) + (x << 8)) >>> 0;
  }
  out64 = hi.toString(16).padStart(8, "0") + lo.toString(16).padStart(8, "0");
} else {
  let h = 0xcbf29ce484222325n;
  for (let i = 0; i < b.length; i++) h = BigInt.asUintN(64, (h ^ BigInt(b[i])) * 0x100000001b3n);
  out64 = h.toString(16).padStart(16, "0");
}
console.log(h32.toString(16).padStart(8, "0"));
console.log(out64);
