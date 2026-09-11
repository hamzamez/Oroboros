// AoS against SoA on the node table, on JavaScript. See gauntlet/go/aossoa.go
// for the question and the design; this is the same three layouts and the same
// two workloads.
//
//   flat   nodes[4*n+f], one plain Array     what the product pass emits here
//   SoA    four plain Arrays
//   AoS    an Array of objects {tag, val, kid, sib}
//
// On this host AoS is NOT the same bytes as flat, unlike Go: an object is a
// pointer to a hidden-class-shaped heap cell, so AoS is g2's "array of object
// pointers" — measured at 2.86x against parallel arrays in August, on a
// different program. That is the claim this re-takes.
//
// PLAIN ARRAYS, because that is what our emitter produces on this host
// (JavaScript declares no int-repr and keeps a packed Array of Smis, which
// jsontok measured 1.15x faster than a Uint8Array anyway).
//
//   node aossoa.mjs --check
//   node aossoa.mjs <case>      one case per process
//
// ONE PROCESS PER CASE: V8 carries optimisation state across benchmarks in a
// process (native-js-2026-08-20), and every surprising JavaScript number in this
// repository has been a method error at least once.

const NMAX = 512;
const DMAX = 32;
const SCAN_N = 65536;

const isNum = (c) =>
  (c >= 48 && c <= 57) || c === 45 || c === 43 || c === 46 || c === 101 || c === 69;
const isAlpha = (c) => c >= 97 && c <= 122;
const isSkip = (c) =>
  c === 32 || c === 9 || c === 10 || c === 13 || c === 58 || c === 44;

function scanString(a, i) {
  let j = i + 1;
  for (;;) {
    if (j >= a.length) return j;
    if (a[j] === 92) { j += 2; continue; }
    if (a[j] === 34) return j + 1;
    j++;
  }
}

// The parse is written ONCE over an accessor object, per layout, so the three
// cannot drift apart in anything but the storage. V8 inlines the closures; if it
// did not, all three would pay the same price, which keeps the comparison fair.
function parse(a, st) {
  const stk = new Array(2 * DMAX).fill(0);
  let i = 0, nn = 1, sp = 0;
  const link = (k) => {
    if (sp < 1) return;
    const lc = stk[2 * (sp - 1) + 1];
    if (lc === 0) st.setKid(stk[2 * (sp - 1)], k); else st.setSib(lc, k);
  };
  for (;;) {
    if (i >= a.length || nn >= NMAX || sp >= DMAX) break;
    const c = a[i];
    if (isSkip(c)) { i++; continue; }
    if (c === 123 || c === 91) {
      st.setTag(nn, c === 123 ? 5 : 4); st.setVal(nn, 0);
      link(nn);
      if (sp >= 1) stk[2 * (sp - 1) + 1] = nn;
      stk[2 * sp] = nn; stk[2 * sp + 1] = 0;
      i++; sp++; nn++;
      continue;
    }
    if (c === 125 || c === 93) { i++; if (sp >= 1) sp--; continue; }
    if (c === 34 || isNum(c) || isAlpha(c)) {
      let tg = 2, ni = 0;
      if (c === 34) ni = scanString(a, i);
      else if (isNum(c)) { tg = 1; let j = i; while (j < a.length && isNum(a[j])) j++; ni = j; }
      else { tg = 3; let j = i; while (j < a.length && isAlpha(a[j])) j++; ni = j; }
      st.setTag(nn, tg); st.setVal(nn, ni - i);
      link(nn);
      if (sp >= 1) stk[2 * (sp - 1) + 1] = nn;
      i = ni; nn++;
      continue;
    }
    i++;
  }
}

function walk(st) {
  const wl = new Array(2 * NMAX).fill(0);
  wl[0] = 1; wl[1] = 1;
  let sp = 1, seen = 0, acc = 0, steps = 0;
  while (sp >= 1 && steps < 2 * NMAX) {
    const n = wl[2 * (sp - 1)], d = wl[2 * (sp - 1) + 1];
    const sb = st.sib(n), kd = st.kid(n);
    sp--;
    if (sb !== 0) { wl[2 * sp] = sb; wl[2 * sp + 1] = d; sp++; }
    if (kd !== 0) { wl[2 * sp] = kd; wl[2 * sp + 1] = d + 1; sp++; }
    seen++; acc += st.tag(n) * d; steps++;
  }
  return seen * 1000 + acc;
}

// ------------------------------------------------------------------ W1

function treeFlat(a) {
  const t = new Array(4 * NMAX).fill(0);
  const st = {
    setTag: (n, v) => { t[4 * n] = v; }, setVal: (n, v) => { t[4 * n + 1] = v; },
    setKid: (n, v) => { t[4 * n + 2] = v; }, setSib: (n, v) => { t[4 * n + 3] = v; },
    tag: (n) => t[4 * n], kid: (n) => t[4 * n + 2], sib: (n) => t[4 * n + 3],
  };
  parse(a, st);
  return walk(st);
}

function treeSoA(a) {
  const tg = new Array(NMAX).fill(0), vl = new Array(NMAX).fill(0);
  const kd = new Array(NMAX).fill(0), sb = new Array(NMAX).fill(0);
  const st = {
    setTag: (n, v) => { tg[n] = v; }, setVal: (n, v) => { vl[n] = v; },
    setKid: (n, v) => { kd[n] = v; }, setSib: (n, v) => { sb[n] = v; },
    tag: (n) => tg[n], kid: (n) => kd[n], sib: (n) => sb[n],
  };
  parse(a, st);
  return walk(st);
}

function treeAoS(a) {
  // Every slot is a real object from the start, with ONE hidden class, so the
  // array stays PACKED_ELEMENTS and every access is monomorphic — the best case
  // an array of objects gets on V8, which is the fair one to measure.
  const ns = new Array(NMAX);
  for (let i = 0; i < NMAX; i++) ns[i] = { tag: 0, val: 0, kid: 0, sib: 0 };
  const st = {
    setTag: (n, v) => { ns[n].tag = v; }, setVal: (n, v) => { ns[n].val = v; },
    setKid: (n, v) => { ns[n].kid = v; }, setSib: (n, v) => { ns[n].sib = v; },
    tag: (n) => ns[n].tag, kid: (n) => ns[n].kid, sib: (n) => ns[n].sib,
  };
  parse(a, st);
  return walk(st);
}

// ------------------------------------------------------------------ W2

const fill = (i) => ({ tag: (i * 7) % 5 + 1, val: i % 97, kid: (i + 1) % SCAN_N, sib: (i * 3) % SCAN_N });

function scanTables() {
  const flat = new Array(4 * SCAN_N).fill(0);
  const tag = new Array(SCAN_N).fill(0), val = new Array(SCAN_N).fill(0);
  const aos = new Array(SCAN_N);
  for (let i = 0; i < SCAN_N; i++) {
    const n = fill(i);
    flat[4 * i] = n.tag; flat[4 * i + 1] = n.val; flat[4 * i + 2] = n.kid; flat[4 * i + 3] = n.sib;
    tag[i] = n.tag; val[i] = n.val;
    aos[i] = n;
  }
  return { flat, tag, val, aos };
}

function scanFlat(t) {
  let s = 0;
  for (let i = 0; i + 3 < t.length; i += 4) if (t[i] === 2) s += t[i + 1];
  return s;
}
function scanSoA(tag, val) {
  let s = 0;
  for (let i = 0; i < tag.length; i++) if (tag[i] === 2) s += val[i];
  return s;
}
function scanAoS(ns) {
  let s = 0;
  for (let i = 0; i < ns.length; i++) if (ns[i].tag === 2) s += ns[i].val;
  return s;
}

// ------------------------------------------------------------------ W3
//
// THE CASE AoS EXISTS FOR — random nodes of a table far past cache, all four
// fields read. W1 has that access pattern at 443 nodes, which is L1-resident, so
// it could not test locality; see gauntlet/go/aossoa.go. Park–Miller indices,
// fixed and precomputed: x*48271 stays below 2^47, well inside a double's exact
// range, so the sequence is the same one Go and Java visit.

const GATHER_N = 1 << 20;
const GATHER_M = 1 << 16;

function gatherIdx() {
  const idx = new Array(GATHER_M).fill(0);
  let x = 1;
  for (let i = 0; i < GATHER_M; i++) { x = (x * 48271) % 2147483647; idx[i] = x & (GATHER_N - 1); }
  return idx;
}
const gfill = (i) => ({ tag: (i * 7) % 5 + 1, val: i % 97, kid: (i + 1) % GATHER_N, sib: (i * 3) % GATHER_N });

function gatherTables() {
  const flat = new Array(4 * GATHER_N).fill(0);
  const tag = new Array(GATHER_N).fill(0), val = new Array(GATHER_N).fill(0);
  const kid = new Array(GATHER_N).fill(0), sib = new Array(GATHER_N).fill(0);
  const aos = new Array(GATHER_N);
  for (let i = 0; i < GATHER_N; i++) {
    const n = gfill(i);
    flat[4 * i] = n.tag; flat[4 * i + 1] = n.val; flat[4 * i + 2] = n.kid; flat[4 * i + 3] = n.sib;
    tag[i] = n.tag; val[i] = n.val; kid[i] = n.kid; sib[i] = n.sib;
    aos[i] = n;
  }
  return { flat, tag, val, kid, sib, aos };
}

function gatherFlat(t, idx) {
  let s = 0;
  for (let i = 0; i < idx.length; i++) { const k = 4 * idx[i]; s += t[k] + t[k + 1] + t[k + 2] + t[k + 3]; }
  return s;
}
function gatherSoA(tag, val, kid, sib, idx) {
  let s = 0;
  for (let i = 0; i < idx.length; i++) { const k = idx[i]; s += tag[k] + val[k] + kid[k] + sib[k]; }
  return s;
}
function gatherAoS(ns, idx) {
  let s = 0;
  for (let i = 0; i < idx.length; i++) { const n = ns[idx[i]]; s += n.tag + n.val + n.kid + n.sib; }
  return s;
}

// ------------------------------------------------------------------ input

function makeDoc(records) {
  let s = '{"items":[';
  for (let r = 0; r < records; r++) {
    if (r > 0) s += ",";
    s += '{"id":1234,"name":"a b\\"c","tags":["x","y","z"],' +
      '"score":-12.5e3,"ok":true,"prev":null,' +
      '"meta":{"depth":2,"flag":false}}';
  }
  return s + "]}";
}
const bytesOf = (t) => Array.from(t, (ch) => ch.charCodeAt(0));

function check() {
  let bad = 0;
  for (const n of [0, 1, 2, 5, 20]) {
    const a = bytesOf(makeDoc(n));
    const f = treeFlat(a), s = treeSoA(a), o = treeAoS(a);
    if (f !== s || f !== o) { console.log(`records=${n}: flat=${f} soa=${s} aos=${o}`); bad++; }
  }
  const T = scanTables();
  let want = 0;
  for (let i = 0; i < SCAN_N; i++) { const n = fill(i); if (n.tag === 2) want += n.val; }
  const f = scanFlat(T.flat), s = scanSoA(T.tag, T.val), o = scanAoS(T.aos);
  if (f !== want || s !== want || o !== want || want === 0) {
    console.log(`scan: flat=${f} soa=${s} aos=${o} want=${want}`); bad++;
  }
  const G = gatherTables(), gi = gatherIdx();
  let gwant = 0;
  for (const k of gi) { const n = gfill(k); gwant += n.tag + n.val + n.kid + n.sib; }
  const gf = gatherFlat(G.flat, gi), gs = gatherSoA(G.tag, G.val, G.kid, G.sib, gi), go = gatherAoS(G.aos, gi);
  if (gf !== gwant || gs !== gwant || go !== gwant || gwant === 0) {
    console.log(`gather: flat=${gf} soa=${gs} aos=${go} want=${gwant}`); bad++;
  }
  console.log(bad === 0 ? "all layouts agree" : `${bad} disagreement(s)`);
  if (bad) process.exit(1);
}

// A SINK THE RESULT ESCAPES INTO, because with loop-invariant inputs and an
// unused result V8 eliminated a whole multiply once (karatsuba-2026-08-30).
let sink = 0;

function bench(name, fn, iters, runs = 7) {
  const w0 = process.hrtime.bigint();
  for (let i = 0; i < 20000; i++) {
    sink ^= fn();
    if (i >= 1000 && Number(process.hrtime.bigint() - w0) > 2e9) break;
  }
  const ts = [];
  for (let r = 0; r < runs; r++) {
    const t0 = process.hrtime.bigint();
    for (let i = 0; i < iters; i++) sink ^= fn();
    ts.push(Number(process.hrtime.bigint() - t0) / iters);
  }
  ts.sort((a, b) => a - b);
  console.log(`${name.padEnd(28)} ${ts[ts.length >> 1].toFixed(1).padStart(12)} ns/op   (sink ${sink & 1})`);
}

const doc = bytesOf(makeDoc(20));
const which = process.argv[2];
if (which === "--check" || which === undefined) check();
else {
  const T = which.startsWith("scan") ? scanTables() : null;
  const G = which.startsWith("gather") ? gatherTables() : null;
  const gi = G ? gatherIdx() : null;
  const CASES = {
    "gather-flat": ["W3 gather flat", () => gatherFlat(G.flat, gi), 20],
    "gather-soa": ["W3 gather SoA", () => gatherSoA(G.tag, G.val, G.kid, G.sib, gi), 20],
    "gather-aos": ["W3 gather AoS objects", () => gatherAoS(G.aos, gi), 20],
    "tree-flat": ["W1 tree  flat", () => treeFlat(doc), 2000],
    "tree-soa": ["W1 tree  SoA", () => treeSoA(doc), 2000],
    "tree-aos": ["W1 tree  AoS objects", () => treeAoS(doc), 2000],
    "scan-flat": ["W2 scan  flat", () => scanFlat(T.flat), 200],
    "scan-soa": ["W2 scan  SoA", () => scanSoA(T.tag, T.val), 200],
    "scan-aos": ["W2 scan  AoS objects", () => scanAoS(T.aos), 200],
  };
  const c = CASES[which];
  if (!c) { console.error(`no such case: ${which}`); process.exit(1); }
  bench(c[0], c[1], c[2]);
}
