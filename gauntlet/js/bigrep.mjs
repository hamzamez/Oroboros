// ARBITRARY PRECISION ON V8: what we emit against what a person writes.
//
// `bigarith.mjs` asks whether OUR limb form beats `BigInt`, which is the R3
// design question. This asks the question the gauntlet always asks: is the
// emitted code at parity with hand-written code on this host.
//
// V8 IS WHERE PARITY IS MOST NEARLY BY CONSTRUCTION AND LEAST SAFE TO ASSUME.
// `BigInt` keeps the operators — `a + b`, `a * b`, `a > b` — so the emitted form
// and the hand-written one are textually almost the same, and it is tempting to
// skip the measurement. Every surprising JavaScript number in this repository
// has been a method error at least once (native-js-2026-08-20 found a module
// namespace lookup at 1.66x, jsontok-2026-08-26 found a per-byte closure at
// 1.5x, karatsuba-2026-08-30 found V8 eliminating the multiply outright), so
// "obviously the same" is exactly the claim that has been wrong here before.
//
// ONE VARIANT PER PROCESS, because V8 carries optimisation state across
// benchmarks in one process — native-js-2026-08-20 recorded that as a
// benchmark-method error that had been inflating every JS number here.
//
//   node bigrep.mjs check
//   node bigrep.mjs <variant> <iters>

import { genFib } from "./gen_bigfib.mjs";
import { genFact } from "./gen_bigfact.mjs";
import { genPower } from "./gen_bigpower.mjs";

// ------------------------------------------------------------ hand-written
//
// What a JavaScript programmer writes: `BigInt` accumulators and an ordinary
// Number counter, converted at the operation. The counter being a Number is the
// point — a BigInt loop counter is what a representation solver produces if it
// promotes everything a bignum operation touches, and it is pure loss.

export function fibHand(n) {
  let a = 0n, b = 1n;
  for (let i = 0; i < n; i++) { const t = a + b; a = b; b = t; }
  return a;
}

export function factHand(n) {
  let acc = 1n;
  for (let i = 2; i <= n; i++) acc *= BigInt(i);
  return acc;
}

export function powerHand(b, e) {
  let acc = 1n, x = BigInt(b);
  for (let k = e; k !== 0; k = Math.trunc(k / 2)) {
    if (k % 2 === 1) acc *= x;
    x *= x;
  }
  return acc;
}

// ------------------------------------------------------------ correctness
//
// Against `BigInt` computed a DIFFERENT way — `**` for the power, a closed
// oracle rather than the same loop under another name.

function check() {
  let bad = 0;
  const eq = (what, got, want) => {
    if (got !== want) { console.log(`FAIL ${what}: ${got} != ${want}`); bad++; }
  };
  for (const n of [0, 1, 2, 10, 50, 90, 100, 200, 300]) {
    eq(`fib(${n})`, genFib(n), fibHand(n));
  }
  eq("fib(100) exact", genFib(100), 354224848179261915075n);
  for (const n of [0, 1, 5, 20, 30, 100, 200]) {
    let want = 1n;
    for (let k = 2n; k <= BigInt(n); k++) want *= k;
    eq(`fact(${n})`, genFact(n), want);
  }
  for (const [b, e] of [[2, 10], [3, 40], [7, 33], [999, 64], [1000, 60], [5, 0]]) {
    eq(`power(${b},${e})`, genPower(b, e), BigInt(b) ** BigInt(e));
  }
  console.log(bad === 0 ? "ok — emitted agrees with hand-written and with BigInt" : `${bad} FAILURES`);
  return bad;
}

// A SINK THE RESULT ESCAPES INTO, and a time budget rather than a fixed
// iteration count. karatsuba-2026-08-30 measured a 16,384-bit product at 48
// ns/op with neither, because V8 eliminated the whole computation.
let sink = 0n;

function bench(fn, iters) {
  for (let i = 0; i < Math.max(iters, 200); i++) sink ^= fn() & 1n;
  const times = [];
  for (let r = 0; r < 7; r++) {
    const t0 = process.hrtime.bigint();
    for (let i = 0; i < iters; i++) sink ^= fn() & 1n;
    times.push(Number(process.hrtime.bigint() - t0) / iters);
  }
  times.sort((a, b) => a - b);
  return times[3];
}

const [variant, iterStr] = process.argv.slice(2);
if (variant === "check") process.exit(check() === 0 ? 0 : 1);

const iters = Number(iterStr || 20000);
const table = {
  "fib-gen": () => genFib(1000),
  "fib-hand": () => fibHand(1000),
  "fact-gen": () => genFact(200),
  "fact-hand": () => factHand(200),
  "power-gen": () => genPower(999, 64),
  "power-hand": () => powerHand(999, 64),
};
const fn = table[variant];
if (!fn) {
  console.log("variants: " + Object.keys(table).join(" ") + " | check");
  process.exit(1);
}
console.log(`${variant.padEnd(12)} ${bench(fn, iters).toFixed(1)} ns/op   (sink ${sink})`);
