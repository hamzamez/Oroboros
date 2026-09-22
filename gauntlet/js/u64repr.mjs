// A VALUE IN [0, 2^64) ON V8: which representation — ADR 0026 (10), u64repr-2026-09-22.
//
// Under ADR 0026 a value proven in [0, 2^64−1] is native on Go and windows, and
// JavaScript has no 64-bit integer. The candidates:
//
//   bigint  plain `BigInt` arithmetic: `v * 256n + BigInt(b)`;
//   asuint  the same, each result wrapped in `BigInt.asUintN(64, …)`. LEGAL BY
//           THE HOMOMORPHISM: ℤ → ℤ/2^64 is a ring map and the result is proven
//           in [0, 2^64), so reducing mod 2^64 changes nothing — and it tells V8
//           the value fits a machine word, which TurboFan can lower;
//   split   two Numbers, the high and low 32 bits, in two locals — what the
//           compiler already does to a product, so it allocates nothing;
//   number  a Number, which is exact only below 2^53 — the baseline a value
//           that fits would get, and NOT correct on this data (the check says so).
//
// Two kernels, each what a u64 value actually does in the next packages:
//
//   decode  `binary.BigEndian.Uint64`: v = Σ b_i · 256^(7−i), then a running
//           maximum under unsigned order — multiply by a small constant, add,
//           compare;
//   digits  `strconv.FormatUint`'s loop: q = v / 10, r = v % 10 until v = 0 —
//           division, which the homomorphism does NOT license, over the whole
//           64-bit range.
//
// ONE VARIANT PER PROCESS (native-js-2026-08-20).
//
//   node u64repr.mjs check
//   node u64repr.mjs <kernel> <variant> <iters>

const N = 1024;
const bytes = new Uint8Array(8 * N);
let seed = 0x9e3779b9;
for (let i = 0; i < bytes.length; i++) {
  seed = (Math.imul(seed, 1103515245) + 12345) >>> 0;
  bytes[i] = seed >>> 24;
}

// ------------------------------------------------------------ decode

function decodeBig() {
  let max = 0n;
  for (let k = 0; k < N; k++) {
    let v = 0n;
    for (let i = 0; i < 8; i++) v = v * 256n + BigInt(bytes[8 * k + i]);
    if (v > max) max = v;
  }
  return max;
}

function decodeAsU() {
  let max = 0n;
  for (let k = 0; k < N; k++) {
    let v = 0n;
    for (let i = 0; i < 8; i++) v = BigInt.asUintN(64, BigInt.asUintN(64, v * 256n) + BigInt(bytes[8 * k + i]));
    if (v > max) max = v;
  }
  return max;
}

// split: v = hi·2^32 + lo, both in [0, 2^32). v·256 + b: lo·256 + b < 2^40,
// exact in a Number; the carry is its top 8 bits.
function decodeSplit() {
  let mhi = 0, mlo = 0;
  for (let k = 0; k < N; k++) {
    let hi = 0, lo = 0;
    for (let i = 0; i < 8; i++) {
      const t = lo * 256 + bytes[8 * k + i];
      const c = Math.floor(t / 4294967296); // `>>> 0` measured slower on V8 (62 vs 39 ns)
      lo = t - c * 4294967296;
      hi = hi * 256 + c; // proven < 2^32 by the range: no reduction needed
    }
    if (hi > mhi || (hi === mhi && lo > mlo)) { mhi = hi; mlo = lo; }
  }
  return BigInt(mhi) * 4294967296n + BigInt(mlo);
}

function decodeNumber() {
  let max = 0;
  for (let k = 0; k < N; k++) {
    let v = 0;
    for (let i = 0; i < 8; i++) v = v * 256 + bytes[8 * k + i];
    if (v > max) max = v;
  }
  return BigInt(max);
}

// ------------------------------------------------------------ digits
//
// The values are decoded once, outside the timing, into the variant's own
// representation, so this measures division alone.

const valsBig = [];
for (let k = 0; k < N; k++) {
  let v = 0n;
  for (let i = 0; i < 8; i++) v = v * 256n + BigInt(bytes[8 * k + i]);
  valsBig.push(v);
}
const valsHi = new Float64Array(N), valsLo = new Float64Array(N), valsNum = new Float64Array(N);
for (let k = 0; k < N; k++) {
  valsHi[k] = Number(valsBig[k] >> 32n);
  valsLo[k] = Number(valsBig[k] & 0xffffffffn);
  valsNum[k] = Number(valsBig[k]);
}

function digitsBig() {
  let s = 0n;
  for (let k = 0; k < N; k++) {
    let v = valsBig[k];
    while (v !== 0n) { s += v % 10n; v /= 10n; }
  }
  return s;
}

function digitsAsU() {
  let s = 0;
  for (let k = 0; k < N; k++) {
    let v = valsBig[k];
    while (v !== 0n) {
      s += Number(BigInt.asUintN(64, v % 10n));
      v = BigInt.asUintN(64, v / 10n);
    }
  }
  return BigInt(s);
}

// split long division by 10: hi = 10·qh + r; (r·2^32 + lo) < 10·2^32 < 2^36.
function digitsSplit() {
  let s = 0;
  for (let k = 0; k < N; k++) {
    let hi = valsHi[k], lo = valsLo[k];
    while (hi !== 0 || lo !== 0) {
      const qh = Math.floor(hi / 10), rh = hi - qh * 10;
      const t = rh * 4294967296 + lo;
      const ql = Math.floor(t / 10);
      s += t - ql * 10;
      hi = qh; lo = ql;
    }
  }
  return BigInt(s);
}

function digitsNumber() {
  let s = 0;
  for (let k = 0; k < N; k++) {
    let v = valsNum[k];
    while (v !== 0) { const q = Math.floor(v / 10); s += v - q * 10; v = q; }
  }
  return BigInt(s);
}

const kernels = {
  decode: { bigint: decodeBig, asuint: decodeAsU, split: decodeSplit, number: decodeNumber },
  digits: { bigint: digitsBig, asuint: digitsAsU, split: digitsSplit, number: digitsNumber },
};

function check() {
  let bad = 0;
  for (const [kname, vs] of Object.entries(kernels)) {
    const want = vs.bigint();
    for (const [vname, f] of Object.entries(vs)) {
      const got = f();
      const ok = got === want;
      if (!ok && vname !== "number") bad++;
      console.log(`${kname} ${vname}: ${ok ? "ok" : `WRONG (${got} != ${want})`}`);
    }
  }
  console.log(bad === 0 ? "every candidate agrees with BigInt (number is expected to be wrong)" : `${bad} FAILURES`);
  return bad;
}

const [, , kernel, variant, iterArg] = process.argv;
if (kernel === "check" || kernel === undefined) {
  process.exit(check());
}
const f = kernels[kernel][variant];
const iters = Number(iterArg || 2000);
let sink;
for (let i = 0; i < Math.min(200, iters); i++) sink = f(); // warm
const reps = [];
for (let r = 0; r < 5; r++) {
  const t0 = process.hrtime.bigint();
  for (let i = 0; i < iters; i++) sink = f();
  const t1 = process.hrtime.bigint();
  reps.push(Number(t1 - t0) / iters / N);
}
reps.sort((a, b) => a - b);
console.log(`${kernel} ${variant} ns/value min ${reps[0].toFixed(2)} median ${reps[2].toFixed(2)}${sink === undefined ? "!" : ""}`);
