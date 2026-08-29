// R3 on JavaScript: V8's BigInt against limb forms we could emit.
//
// Companion to gauntlet/go/bigarith.go, which found ours beating math/big at
// every size on Go. JS is the host expected to give the OPPOSITE answer, and
// the reason is structural rather than a matter of tuning:
//
//   - a JS number is a double, so integer arithmetic is exact only to 2^53;
//   - the bitwise operators coerce to INT32, so anything bitwise is 32-bit;
//   - there is no 64x64 -> 128 multiply, and `Math.imul` gives only the LOW
//     32 bits of a 32x32 product.
//
// So our limbs cannot be 64 bits here. They have to be small enough that a
// product plus accumulated carry stays exact, which is what makes this host
// different rather than merely slower.
//
// THREE LIMB BASES, chosen by what each operation can hold:
//
//   base 2^24, plain float arithmetic. A limb times a small multiplier plus
//     carry stays under 2^53, so `%` and division are exact. The classic
//     JS-bignum choice (jsbn uses 28-bit, node-forge 26-bit).
//   base 2^16, BITWISE. acc[i] < 2^16 times k < 2^11 is under 2^27, so the
//     whole multiply-and-carry chain fits in an int32 and the carry is a
//     shift and a mask. Half the limb width of the float form, so twice the
//     iterations — which is the trade this file exists to measure.
//   base 2^30, BITWISE, for addition only. Two 30-bit limbs plus a carry is
//     under 2^31, so it fits int32 exactly. Multiplication cannot use this
//     base at all, which is why the two workloads take different ones.
//
// Storage is measured both ways for the float form, because
// jsontok-2026-08-26 found a plain packed Array 1.15x FASTER than a
// Uint8Array on V8 and that result should not be assumed to generalise.
//
// Run one variant per process: V8 carries optimization state across
// benchmarks in one process, which native-js-2026-08-20 recorded as a
// benchmark-method error that had been inflating every JS number here.
//
//   node bigarith.mjs <variant> <n> <iters>
//   node bigarith.mjs check                 # correctness against BigInt

// ------------------------------------------------------------------ BigInt

export function factBigInt(n) {
  let acc = 1n;
  const N = BigInt(n);
  for (let k = 2n; k <= N; k++) acc *= k;
  return acc;
}

export function fibBigInt(n) {
  let a = 0n, b = 1n;
  for (let i = 0; i < n; i++) { const t = a + b; a = b; b = t; }
  return a;
}

// -------------------------------------------------- base 2^24, plain floats

const B24 = 16777216; // 2^24

// factF24 multiplies in place by a small value. `acc[i] * k + carry` is at most
// 2^24 * 2^11 + 2^11, comfortably inside 2^53, so `%` and `/` are exact.
export function factF24(n, acc) {
  acc.fill(0);
  acc[0] = 1;
  let used = 1;
  for (let k = 2; k <= n; k++) {
    let carry = 0;
    for (let i = 0; i < used; i++) {
      const t = acc[i] * k + carry;
      acc[i] = t % B24;
      carry = Math.floor(t / B24);
    }
    while (carry > 0) { acc[used++] = carry % B24; carry = Math.floor(carry / B24); }
  }
  return used;
}

// fibF24 adds. A sum of two limbs plus a carry is under 2^25, so the carry is
// a comparison rather than a division.
export function fibF24(n, a, b, t) {
  a.fill(0); b.fill(0); t.fill(0);
  b[0] = 1;
  let ua = 1, ub = 1;
  for (let i = 0; i < n; i++) {
    const u = ua > ub ? ua : ub;
    let carry = 0;
    for (let j = 0; j < u; j++) {
      const s = a[j] + b[j] + carry;
      if (s >= B24) { t[j] = s - B24; carry = 1; } else { t[j] = s; carry = 0; }
    }
    let ut = u;
    if (carry) { t[u] = carry; ut = u + 1; }
    const x = a; a = b; b = t; t = x;
    ua = ub; ub = ut;
  }
  return [a, ua];
}

// ------------------------------------------ base 2^20, plain: the CONTROL
//
// Identical to factF24 but for the base, and the base is chosen so that
// `acc[i] * k` stays under 2^31 for every n measured here (2^20 * 2000 = 2^31
// exactly at the top, and under it for n <= 2000). If the f24 cliff is V8
// leaving int32 rather than leaving exactness, this form must not have one —
// both stay far inside 2^53, and only this one stays inside int32.
const B20 = 1048576; // 2^20

export function factF20(n, acc) {
  acc.fill(0);
  acc[0] = 1;
  let used = 1;
  for (let k = 2; k <= n; k++) {
    let carry = 0;
    for (let i = 0; i < used; i++) {
      const t = acc[i] * k + carry;
      acc[i] = t % B20;
      carry = Math.floor(t / B20);
    }
    while (carry > 0) { acc[used++] = carry % B20; carry = Math.floor(carry / B20); }
  }
  return used;
}

// ------------------------------------------------------ base 2^16, BITWISE

// factB16 keeps the whole multiply-and-carry chain in int32: acc[i] is under
// 2^16, k is under 2^11 for every n this file measures, so the product plus
// carry is under 2^28 and `>>> 16` is exact.
export function factB16(n, acc) {
  if (n >= 2048) throw new Error("factB16 needs k < 2^11 to stay in int32");
  acc.fill(0);
  acc[0] = 1;
  let used = 1;
  for (let k = 2; k <= n; k++) {
    let carry = 0;
    for (let i = 0; i < used; i++) {
      const t = acc[i] * k + carry;
      acc[i] = t & 0xffff;
      carry = t >>> 16;
    }
    while (carry > 0) { acc[used++] = carry & 0xffff; carry = carry >>> 16; }
  }
  return used;
}

// ------------------------------------------------------ base 2^30, BITWISE

// fibB30 is the widest bitwise addition JavaScript allows: two 30-bit limbs
// plus a carry is under 2^31, which is exactly int32's range.
export function fibB30(n, a, b, t) {
  a.fill(0); b.fill(0); t.fill(0);
  b[0] = 1;
  let ua = 1, ub = 1;
  for (let i = 0; i < n; i++) {
    const u = ua > ub ? ua : ub;
    let carry = 0;
    for (let j = 0; j < u; j++) {
      const s = a[j] + b[j] + carry;
      t[j] = s & 0x3fffffff;
      carry = s >>> 30;
    }
    let ut = u;
    if (carry) { t[u] = carry; ut = u + 1; }
    const x = a; a = b; b = t; t = x;
    ua = ub; ub = ut;
  }
  return [a, ua];
}

// ------------------------------------------------------------ big x big
//
// The case the other workloads avoid. Base 2^15 rather than 2^16, because the
// accumulation here is `out[i+j] + a[i]*b[j] + carry`: at 2^15 the product is
// under 2^30 and the whole sum stays inside int32, which §5a showed is the
// boundary that actually matters on V8. At 2^16 the product alone is 2^32.
const B15 = 32768;

export function mulB15(a, b, out) {
  out.fill(0);
  for (let i = 0; i < a.length; i++) {
    let carry = 0;
    const ai = a[i];
    for (let j = 0; j < b.length; j++) {
      const t = ai * b[j] + out[i + j] + carry;
      out[i + j] = t & 0x7fff;
      carry = t >>> 15;
    }
    out[i + b.length] = carry;
  }
  return out;
}

export function limbsOf(n, seed, w) {
  const out = new Int32Array(n);
  let x = seed;
  for (let i = 0; i < n; i++) {
    x = (Math.imul(x, 1103515245) + 12345) >>> 0;
    out[i] = (x >>> 8) & ((1 << w) - 1);
  }
  out[n - 1] |= 1 << (w - 1);
  return out;
}

export function limbsToBigInt(l, w) {
  let out = 0n;
  const W = BigInt(w);
  for (let i = l.length - 1; i >= 0; i--) out = (out << W) | BigInt(l[i] >>> 0);
  return out;
}

// ------------------------------------------------------------------ sizing

// The limb count a compiler would have to derive BEFORE the loop. A product
// has at most len(a)+len(b) limbs and a sum at most max+1, so every bignum
// result's size is a function of its operands' sizes — which is why a bignum
// needs no growable storage (growth.md, bigarith-2026-08-28 §3b).
export function factBits(n) {
  let b = 1;
  for (let k = 2; k <= n; k++) b += 32 - Math.clz32(k);
  return b;
}
export const fibBits = (n) => Math.ceil(n * 0.695) + 2;

const limbs = (bits, w) => Math.ceil(bits / w) + 2;

// ----------------------------------------------------------------- harness

function toBig(l, used, w) {
  let out = 0n;
  const W = BigInt(w);
  for (let i = used - 1; i >= 0; i--) out = (out << W) | BigInt(l[i]);
  return out;
}

function check() {
  let bad = 0;
  for (const n of [0, 1, 2, 5, 20, 21, 50, 200, 2000]) {
    const want = factBigInt(n);
    const c24 = limbs(factBits(n), 24), c16 = limbs(factBits(n), 16);
    for (const [name, f, arr, w] of [
      ["f24-array", factF24, new Array(c24).fill(0), 24],
      ["f24-typed", factF24, new Float64Array(c24), 24],
      ["b16-typed", factB16, new Int32Array(c16), 16],
      ["f20-array", factF20, new Array(limbs(factBits(n), 20)).fill(0), 20],
    ]) {
      if (name === "b16-typed" && n >= 2048) continue;
      const used = f(n, arr);
      const got = toBig(arr, used, w);
      if (got !== want) { console.log(`FAIL fact(${n}) ${name}: ${got} != ${want}`); bad++; }
    }
  }
  for (const n of [0, 1, 2, 10, 93, 94, 300, 1000]) {
    const want = fibBigInt(n);
    for (const [name, f, w] of [["f24", fibF24, 24], ["b30", fibB30, 30]]) {
      const c = limbs(fibBits(n), w);
      const mk = name === "f24" ? (() => new Float64Array(c)) : (() => new Int32Array(c));
      const [res, used] = f(n, mk(), mk(), mk());
      const got = toBig(res, used, w);
      if (got !== want) { console.log(`FAIL fib(${n}) ${name}: ${got} != ${want}`); bad++; }
    }
  }
  // The workload must actually leave the window, or this measures machine
  // arithmetic with extra steps. 21! and fib(93) are the first past 2^63.
  if (factBigInt(20) >= 2n ** 63n || factBigInt(21) < 2n ** 63n) { console.log("FAIL: 21! crossover"); bad++; }
  if (fibBigInt(92) >= 2n ** 63n || fibBigInt(93) < 2n ** 63n) { console.log("FAIL: fib(93) crossover"); bad++; }
  for (const bits of [64, 128, 256, 1024]) {
    const n = Math.ceil(bits / 15);
    const a = limbsOf(n, 12345, 15), b = limbsOf(n, 67890, 15);
    const got = limbsToBigInt(mulB15(a, b, new Int32Array(2 * n + 1)), 15);
    const want = limbsToBigInt(a, 15) * limbsToBigInt(b, 15);
    if (got !== want) { console.log(`FAIL mulB15 ${bits}: ${got} != ${want}`); bad++; }
  }
  console.log(bad === 0 ? "ok — every limb form agrees with BigInt" : `${bad} FAILURES`);
  return bad;
}

function bench(fn, iters) {
  for (let i = 0; i < Math.max(iters, 200); i++) fn();
  const times = [];
  for (let r = 0; r < 7; r++) {
    const t0 = process.hrtime.bigint();
    for (let i = 0; i < iters; i++) fn();
    times.push(Number(process.hrtime.bigint() - t0) / iters);
  }
  times.sort((a, b) => a - b);
  return times[3];
}

const [variant, nStr, iterStr] = process.argv.slice(2);
if (variant === "check") process.exit(check() === 0 ? 0 : 1);

const n = Number(nStr), iters = Number(iterStr);
let fn;
switch (variant) {
  case "fact-bigint": fn = () => factBigInt(n); break;
  case "fact-f24-array": { const a = new Array(limbs(factBits(n), 24)).fill(0); fn = () => factF24(n, a); break; }
  case "fact-f24-typed": { const a = new Float64Array(limbs(factBits(n), 24)); fn = () => factF24(n, a); break; }
  case "fact-f20-array": { const a = new Array(limbs(factBits(n), 20)).fill(0); fn = () => factF20(n, a); break; }
  case "fact-b16-typed": { const a = new Int32Array(limbs(factBits(n), 16)); fn = () => factB16(n, a); break; }
  case "fib-bigint": fn = () => fibBigInt(n); break;
  case "fib-f24-typed": { const c = limbs(fibBits(n), 24); const x = new Float64Array(c), y = new Float64Array(c), z = new Float64Array(c); fn = () => fibF24(n, x, y, z); break; }
  case "fib-f24-array": { const c = limbs(fibBits(n), 24); const x = new Array(c).fill(0), y = new Array(c).fill(0), z = new Array(c).fill(0); fn = () => fibF24(n, x, y, z); break; }
  case "fib-b30-array": { const c = limbs(fibBits(n), 30); const x = new Array(c).fill(0), y = new Array(c).fill(0), z = new Array(c).fill(0); fn = () => fibB30(n, x, y, z); break; }
  case "fib-b30-typed": { const c = limbs(fibBits(n), 30); const x = new Int32Array(c), y = new Int32Array(c), z = new Int32Array(c); fn = () => fibB30(n, x, y, z); break; }
  case "mul-bigint": {
    const nb = Math.ceil(n / 15);
    const x = limbsToBigInt(limbsOf(nb, 12345, 15), 15), y = limbsToBigInt(limbsOf(nb, 67890, 15), 15);
    fn = () => x * y; break;
  }
  case "mul-b15": {
    const nb = Math.ceil(n / 15);
    const x = limbsOf(nb, 12345, 15), y = limbsOf(nb, 67890, 15), o = new Int32Array(2 * nb + 1);
    fn = () => mulB15(x, y, o); break;
  }
  default: console.error("unknown variant " + variant); process.exit(2);
}
console.log(`${variant.padEnd(18)} n=${String(n).padEnd(5)} ${bench(fn, iters).toFixed(1)} ns/op`);
