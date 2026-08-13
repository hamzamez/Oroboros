import * as g from "./gauntlet.mjs";

const NVEC = 1 << 16;
const NSMALL = 1024;

// Median of `runs` timings, each averaging `iters` calls, after a warmup pass
// long enough for V8 to reach its optimizing tier.
function bench(name, fn, iters, runs = 7) {
  for (let i = 0; i < Math.max(iters, 20000); i++) fn();
  const times = [];
  for (let r = 0; r < runs; r++) {
    const t0 = process.hrtime.bigint();
    for (let i = 0; i < iters; i++) fn();
    const t1 = process.hrtime.bigint();
    times.push(Number(t1 - t0) / iters);
  }
  times.sort((a, b) => a - b);
  const med = times[times.length >> 1];
  results.push([name, med]);
  return med;
}

const results = [];

const vecA = g.makeVec(NVEC, 1);
const vecB = g.makeVec(NVEC, 2);
const arrA = g.makeVecArray(NVEC, 1);
const arrB = g.makeVecArray(NVEC, 2);
const smallA = g.makeVec(NSMALL, 1);
const smallB = g.makeVec(NSMALL, 2);
const smallArrA = g.makeVecArray(NSMALL, 1);
const smallArrB = g.makeVecArray(NSMALL, 2);
const ptsAoS = g.makePointsAoS(NVEC, 4);
const ptsX = g.makeVec(NVEC, 41);
const ptsY = g.makeVec(NVEC, 42);
const text = g.makeText(NVEC, 5);

// G1 — full size, then L1-resident
bench("G1 dotTyped        n=65536", () => g.dotTyped(vecA, vecB), 200);
bench("G1 dotArray        n=65536", () => g.dotArray(arrA, arrB), 200);
bench("G1 dotTyped        n=1024 ", () => g.dotTyped(smallA, smallB), 20000);
bench("G1 dotArray        n=1024 ", () => g.dotArray(smallArrA, smallArrB), 20000);
bench("G1 dotUnordered4   n=1024 ", () => g.dotUnordered4(smallA, smallB), 20000);

// G2 — the array-of-structs question
bench("G2 centroidAoS     n=65536", () => g.centroidAoS(ptsAoS), 200);
bench("G2 centroidSoA     n=65536", () => g.centroidSoA(ptsX, ptsY), 200);
bench("G2 boundsSoA       n=65536", () => g.boundsSoA(ptsX, ptsY), 200);

// G3 — callback parameter
bench("G3 sumMono         n=1024 ", () => g.sumMono(smallA), 20000);
bench("G3 sumFold         n=1024 ", () => g.sumFold(smallA), 20000);
bench("G3 countPosMono    n=1024 ", () => g.countPositiveMono(smallA), 20000);
bench("G3 countPosFold    n=1024 ", () => g.countPositiveFold(smallA), 20000);

// G4 — must use Map
bench("G4 wordCountMap    n=65536", () => g.wordCountMap(text), 30);
bench("G4 wordCountObject n=65536", () => g.wordCountObject(text), 30);

// G6 — closures
const ops = g.buildOps();
let k = 0;
bench("G6 buildOps              ", () => g.buildOps(), 200000);
bench("G6 runOp                 ", () => g.runOp(ops, (k++) % 3, 7), 500000);
bench("G6 makeScaler            ", () => g.makeScaler(k++), 200000);

const w = Math.max(...results.map((r) => r[0].length));
for (const [name, ns] of results) {
  console.log(`${name.padEnd(w)}  ${ns.toFixed(2).padStart(12)} ns/op`);
}
