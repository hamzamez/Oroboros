// RE-TAKING THE MAP MEASUREMENT, with the harness the Karatsuba work forced.
//
// growth.md says three of the four numbers a map design would rest on are
// stale, and this is the oldest: the first baseline measured V8's `Map` at
// 3.25x SLOWER than a null-prototype `Object`, and CLAUDE.md has quoted it ever
// since. It is also exactly the kind of number this repository keeps getting
// wrong — karatsuba-2026-08-30 §3b found V8's `BigInt` measurement wrong twice
// in one afternoon, once from the iteration count and once from dead-code
// elimination.
//
// So the method here is the corrected one:
//
//   - ONE VARIANT PER PROCESS. V8 carries optimization state across benchmarks
//     (native-js-2026-08-20).
//   - TIME-BUDGETED, not count-budgeted. A fixed count gave a 4x spread on
//     identical work.
//   - THE RESULT ESCAPES into a sink that is read afterwards, or V8 eliminates
//     the whole loop.
//
// It also separates two things the original conflated. `wordCount*` SPLITS THE
// TEXT INSIDE the timed region, so a large part of what was measured is
// `String.split` and not the dictionary at all. The `-pre` variants take the
// words already split.
//
//   node maps.mjs check
//   node maps.mjs <variant>

function makeWords(n, seed) {
  // A deterministic Zipf-ish corpus: a few hundred distinct words, most hits
  // landing on the common ones, which is what a word count actually sees.
  const out = new Array(n);
  let x = seed >>> 0;
  for (let i = 0; i < n; i++) {
    x = (Math.imul(x, 1103515245) + 12345) >>> 0;
    const r = (x >>> 8) % 1000;
    const id = r < 700 ? r % 20 : r % 500;
    out[i] = "w" + id;
  }
  return out;
}

export function tallyMap(ws) {
  const m = new Map();
  for (let i = 0; i < ws.length; i++) {
    const w = ws[i];
    m.set(w, (m.get(w) ?? 0) + 1);
  }
  return m;
}

export function tallyObject(ws) {
  const m = Object.create(null);
  for (let i = 0; i < ws.length; i++) {
    const w = ws[i];
    m[w] = (m[w] ?? 0) + 1;
  }
  return m;
}

// A plain `{}` rather than a null-prototype one, because that is what a person
// writes and the prototype chain is the thing said to cost.
export function tallyPlain(ws) {
  const m = {};
  for (let i = 0; i < ws.length; i++) {
    const w = ws[i];
    m[w] = (m[w] ?? 0) + 1;
  }
  return m;
}

// INTEGER KEYS, which is what growth.md proposes building first. A Map keyed by
// number against an array used as a sparse dictionary.
export function tallyMapInt(ks) {
  const m = new Map();
  for (let i = 0; i < ks.length; i++) {
    const k = ks[i];
    m.set(k, (m.get(k) ?? 0) + 1);
  }
  return m;
}

export function tallyObjInt(ks) {
  const m = Object.create(null);
  for (let i = 0; i < ks.length; i++) {
    const k = ks[i];
    m[k] = (m[k] ?? 0) + 1;
  }
  return m;
}

// ----------------------------------------------------------------- harness

let sink = null;

function bench(fn, ms = 500) {
  let t = Date.now();
  while (Date.now() - t < ms) sink = fn();
  const times = [];
  for (let r = 0; r < 5; r++) {
    let n = 0, el;
    const t0 = process.hrtime.bigint();
    do { sink = fn(); n++; el = Number(process.hrtime.bigint() - t0); } while (el < ms * 1e6);
    times.push(el / n);
  }
  times.sort((a, b) => a - b);
  return times[2];
}

const N = 1 << 16;
const words = makeWords(N, 12345);
const ints = words.map((w) => Number(w.slice(1)));

if (process.argv[2] === "check") {
  const a = tallyMap(words), b = tallyObject(words), c = tallyPlain(words);
  let bad = 0;
  for (const [k, v] of a) {
    if (b[k] !== v || c[k] !== v) { console.log("FAIL " + k); bad++; }
  }
  if (Object.keys(b).length !== a.size) { console.log("FAIL size"); bad++; }
  const d = tallyMapInt(ints), e = tallyObjInt(ints);
  for (const [k, v] of d) if (e[k] !== v) { console.log("FAIL int " + k); bad++; }
  console.log(bad === 0
    ? `ok — all forms agree; ${N} words, ${a.size} distinct`
    : `${bad} FAILURES`);
  process.exit(bad === 0 ? 0 : 1);
}

const variants = {
  "map": () => tallyMap(words),
  "object": () => tallyObject(words),
  "plain": () => tallyPlain(words),
  "map-int": () => tallyMapInt(ints),
  "object-int": () => tallyObjInt(ints),
};
const v = process.argv[2];
if (!variants[v]) { console.error("unknown variant " + v); process.exit(2); }
const ns = bench(variants[v]);
if (sink === undefined) console.log("unreachable");
console.log(`${v.padEnd(12)} ${ns.toFixed(0).padStart(9)} ns/op   ${(ns / N).toFixed(1)} ns/word`);
