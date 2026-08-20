// The gauntlet on the NATIVE JavaScript target, against hand-written JS.
//
// JS is the most hostile host in the set and the reason ADR 0004 wanted it
// second: no integers, no structs, no int64. Everything measured here was
// emitted from examples/native/*-js.oro by `cmd/gen -name=native`.
//
//   node native.mjs --check          correctness only
//   node native.mjs --list           the case names
//   node native.mjs <case>           run one case
//   pwsh native.ps1                  every case, one pinned process each
//
// ONE PROCESS PER COMPARISON, and that is not fastidiousness. Running every
// benchmark in a single process gave contradictory answers on consecutive
// runs — native `centroid` at 32,902 ns and then 236,497 ns, hand-written
// `sumMono` at 32,982 and then 135,855 — because V8's optimization state,
// inline caches and GC all carry across benchmarks in one process, and this
// machine is bimodal on top of that (native-gauntlet-2026-08-20 §9).

import * as g from "./gauntlet.mjs";
// NAMED imports for every hand-written reference, and this is not a style
// choice either. `g.dotTyped(a, b)` measures 49,918 ns and `dotTyped(a, b)` —
// the same function, in the same module — measures 30,063. A 1.66x penalty for
// the property load, which blocks the call from being inlined into the
// benchmark closure. The existing harness (bench.mjs) calls everything through
// `g.`, so a generated module imported by name looks 1.66x faster than
// identical hand-written code for no reason at all. Both sides must be called
// the same way.
import { dotTyped, centroidSoA, sumMono, wordCountMap, wordCountObject } from "./gauntlet.mjs";
import { nativeDot } from "./gen_dot_native.mjs";
import { nativeFindFirst } from "./gen_search_native.mjs";
import { nativeCentroid } from "./gen_centroid_native.mjs";
import { nativeSmooth, nativeSmoothInto } from "./gen_smooth_native.mjs";
import { nativeTally } from "./gen_wordcount_native.mjs";
import { nativeSumOf, nativeWordTally } from "./gen_generic_native.mjs";

const N = 1 << 16;

// Warmup is bounded by time as well as by count: 20000 warmup calls is 100
// seconds for a 5 ms wordcount. V8 reaches its optimizing tier after a few
// thousand calls, so warm up for at least 1000 and stop at 20000 or 2 seconds.
function bench(name, fn, iters, runs = 7) {
  const w0 = process.hrtime.bigint();
  for (let i = 0; i < 20000; i++) {
    fn();
    if (i >= 1000 && Number(process.hrtime.bigint() - w0) > 2e9) break;
  }
  const ts = [];
  for (let r = 0; r < runs; r++) {
    const t0 = process.hrtime.bigint();
    for (let i = 0; i < iters; i++) fn();
    ts.push(Number(process.hrtime.bigint() - t0) / iters);
  }
  ts.sort((a, b) => a - b);
  console.log(`${name.padEnd(38)} ${ts[ts.length >> 1].toFixed(1).padStart(12)} ns/op`);
}

const vecA = g.makeVec(N, 1);
const vecB = g.makeVec(N, 2);
const ptsX = g.makeVec(N, 41);
const ptsY = g.makeVec(N, 42);
const text = g.makeText(N, 5);

const searchArr = new Float64Array(100000);
for (let i = 0; i < searchArr.length; i++) searchArr[i] = i;

function findFirstRef(xs, k) {
  for (let i = 0; i < xs.length; i++) if (xs[i] > k) return i;
  return -1;
}

const stSrc = g.makeVec(N, 7);
const stDst = new Float64Array(N);
function smooth(d, s) {
  for (let i = 1; i < s.length - 1; i++) d[i] = (s[i - 1] + s[i] + s[i + 1]) / 3;
}
function smoothNoAlias(d, s) {
  let a = s[0], b = s[1];
  for (let i = 1; i < s.length - 1; i++) {
    const c = s[i + 1];
    d[i] = (a + b + c) / 3;
    a = b; b = c;
  }
}
function smoothFresh(s) {
  const d = new Float64Array(s.length);
  for (let i = 1; i < s.length - 1; i++) d[i] = (s[i - 1] + s[i] + s[i + 1]) / 3;
  return d;
}

// ------------------------------------------------------------- correctness
// No number below means anything until every emitted function agrees with the
// hand-written one.

function eq(what, a, b) {
  if (a !== b) throw new Error(`${what}: native ${a}, hand-written ${b}`);
}

function check() {
  eq("dot", nativeDot(vecA, vecB), dotTyped(vecA, vecB));

  let ax = 0.0, ay = 0.0;
  for (let i = 0; i < ptsX.length; i++) { ax += ptsX[i]; ay += ptsY[i]; }
  eq("centroid", nativeCentroid(ptsX, ptsY), ax + ay);

  for (const k of [-1, 5, 99998, 1e9]) {
    eq(`search k=${k}`, nativeFindFirst(searchArr, k), findFirstRef(searchArr, k));
  }

  const want = wordCountObject(text);
  const got = nativeTally(text);
  eq("wordcount key count", Object.keys(got).length, Object.keys(want).length);
  for (const k of Object.keys(want)) eq(`wordcount ${k}`, got[k], want[k]);

  let s = 0.0;
  for (let i = 0; i < vecA.length; i++) s += vecA[i];
  eq("generic sum", nativeSumOf(vecA), s);
  const gt = nativeWordTally(text);
  for (const k of Object.keys(want)) eq(`generic tally ${k}`, gt[k], want[k]);

  // The stencil indexes differently by construction: ours writes j in
  // [0, len-2) reading a[j..j+2]; the hand-written one writes i in [1, len-1)
  // reading src[i-1..i+1]. Same values, shifted by one.
  const src = g.makeVec(1 << 14, 7);
  const ref = new Float64Array(src.length);
  for (let i = 1; i < src.length - 1; i++) ref[i] = (src[i - 1] + src[i] + src[i + 1]) / 3;
  const out = nativeSmooth(src);
  eq("stencil length", out.length, src.length - 2);
  for (let i = 1; i < src.length - 1; i++) eq(`stencil ${i}`, out[i - 1], ref[i]);
  const into = new Float64Array(src.length);
  nativeSmoothInto(into, src);
  for (let i = 1; i < src.length - 1; i++) eq(`stencilInto ${i}`, into[i - 1], ref[i]);

  console.log("all emitted functions agree with hand-written");
}

// ------------------------------------------------------------- the cases

const CASES = {
  "g1-dot-hand":        ["G1 dot                 hand",   () => dotTyped(vecA, vecB), 200],
  "g1-dot-native":      ["G1 dot                 NATIVE", () => nativeDot(vecA, vecB), 200],

  "g2-centroid-hand":   ["G2 centroid            hand",   () => centroidSoA(ptsX, ptsY), 200],
  "g2-centroid-native": ["G2 centroid            NATIVE", () => nativeCentroid(ptsX, ptsY), 200],

  "g3-sum-hand":        ["G3 sum                 hand",   () => sumMono(vecA), 200],
  "g3-sum-native":      ["G3 sum                 NATIVE", () => nativeSumOf(vecA), 200],

  // The parasite test. Map is carried because baseline R4 measured it 3.25x
  // SLOWER than a null-prototype object for string keys, refuting g4's own
  // assumption that Map was the right choice.
  "g4-map-hand":        ["G4 wordcount Map       hand",   () => wordCountMap(text), 20],
  "g4-object-hand":     ["G4 wordcount Object    hand",   () => wordCountObject(text), 20],
  "g4-tally-native":    ["G4 wordcount           NATIVE", () => nativeTally(text), 20],
  "g4-generic-native":  ["G4 wordcount generic   NATIVE", () => nativeWordTally(text), 20],

  "s-early-hand":       ["S  findFirst early     hand",   () => findFirstRef(searchArr, 5), 200000],
  "s-early-native":     ["S  findFirst early     NATIVE", () => nativeFindFirst(searchArr, 5), 200000],
  "s-late-hand":        ["S  findFirst late      hand",   () => findFirstRef(searchArr, 99998), 200],
  "s-late-native":      ["S  findFirst late      NATIVE", () => nativeFindFirst(searchArr, 99998), 200],

  // ADR 0013 measured 2.01x on JavaScript — WORSE than Go's 1.79x, which is
  // what GC pressure predicts and which the ADR named as its fourth reopening
  // trigger. Both shapes are carried, as on Go.
  "g7-alloc-hand":      ["G7 stencil alloc       hand",   () => smoothFresh(stSrc), 200],
  "g7-alloc-native":    ["G7 stencil alloc       NATIVE", () => nativeSmooth(stSrc), 200],
  "g7-reuse-hand":      ["G7 stencil reuse       hand",   () => smooth(stDst, stSrc), 200],
  "g7-reuse-noalias":   ["G7 stencil reuse       hand/noalias", () => smoothNoAlias(stDst, stSrc), 200],
  "g7-reuse-native":    ["G7 stencil reuse       NATIVE", () => nativeSmoothInto(stDst, stSrc), 200],
};

const which = process.argv[2];
if (which === "--list") {
  console.log(Object.keys(CASES).join("\n"));
} else if (which === "--check" || which === undefined) {
  check();
} else {
  const c = CASES[which];
  if (!c) { console.error(`no such case: ${which}`); process.exit(1); }
  bench(c[0], c[1], c[2]);
}
