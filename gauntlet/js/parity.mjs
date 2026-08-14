// Generated versus hand-written, JavaScript.
import { makeVec } from "./gauntlet.mjs";
import { genDot } from "./generated.mjs";
import { genFilterSum } from "./generated_filter.mjs";

// Hand-written references.
function dotRef(xs, ys) {
  let acc = 0.0;
  for (let i = 0; i < xs.length; i++) acc += xs[i] * ys[i];
  return acc;
}
function filterSumRef(a) {          // binds the element once
  let acc = 0.0;
  for (let i = 0; i < a.length; i++) { const x = a[i]; if (x > 0) acc += x; }
  return acc;
}
function filterSumDup(a) {          // reads it twice
  let acc = 0.0;
  for (let i = 0; i < a.length; i++) { if (a[i] > 0) acc += a[i]; }
  return acc;
}
function filterSumTernary(a) {      // the shape the emitter produced
  let acc = 0.0;
  for (let i = 0; i < a.length; i++) acc = a[i] > 0 ? acc + a[i] : acc;
  return acc;
}

const N = 1024;
const A = makeVec(N, 1), B = makeVec(N, 2);

// Correctness first.
const eq = (x, y, w) => { if (x !== y) throw new Error(`${w}: ${x} !== ${y}`); };
eq(genDot(A, B), dotRef(A, B), "dot");
eq(genFilterSum(A), filterSumRef(A), "filter");
console.log("correctness: generated agrees with hand-written\n");

function bench(name, fn, iters = 20000, runs = 9) {
  for (let i = 0; i < iters; i++) fn();
  const ts = [];
  for (let r = 0; r < runs; r++) {
    const t0 = process.hrtime.bigint();
    for (let i = 0; i < iters; i++) fn();
    ts.push(Number(process.hrtime.bigint() - t0) / iters);
  }
  ts.sort((a, b) => a - b);
  console.log(`${name.padEnd(30)} ${ts[runs >> 1].toFixed(1).padStart(8)} ns/op`);
}

bench("dot  hand-written", () => dotRef(A, B));
bench("dot  GENERATED", () => genDot(A, B));
console.log();
bench("filter  hand-written (bind)", () => filterSumRef(A));
bench("filter  hand-written (dup)", () => filterSumDup(A));
bench("filter  hand-written (ternary)", () => filterSumTernary(A));
bench("filter  GENERATED", () => genFilterSum(A));
