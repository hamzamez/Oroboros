// A JSON tree on JavaScript: emitted against hand-written.
//
// THREE FORMS, and the first two are the question:
//
//   rec    recursive descent into linked objects — what a person writes
//   flat   a flat node table plus indices with an explicit stack — our shape
//   NATIVE emitted from the .oro
//
// `rec` against `flat` prices the REPRESENTATION on this host, and it is not
// the same question as on Go. V8 allocates in a bump-pointer nursery and
// collects short-lived objects almost for free, so the 2.52x the flat form won
// on Go is a measurement rather than a principle (ADR 0008), and it has to be
// taken again here.
//
// `flat` against NATIVE prices our code generation with the representation held
// fixed.
//
//   node jsontree.mjs --check     correctness only
//   node jsontree.mjs <case>      run one case
//
// ONE PROCESS PER COMPARISON, and NAMED imports on both sides — V8 carries
// optimization state across benchmarks in a process, and a namespace property
// load costs 1.66x (native-js-2026-08-20).

import { nativeMeasure } from "./gen_jsontree_native.mjs";

const NMAX = 512;
const DMAX = 32;

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

// ------------------------------------------------------------------- boxed

function skip(a, i) {
  while (i < a.length && isSkip(a[i])) i++;
  return i;
}

function parseValue(a, i) {
  if (i >= a.length) return [null, i];
  const c = a[i];
  if (c === 123 || c === 91) {
    const n = { tag: c === 123 ? 5 : 4, val: 0, kid: null, sib: null };
    i++;
    let last = null;
    for (;;) {
      i = skip(a, i);
      if (i >= a.length || a[i] === 125 || a[i] === 93) {
        if (i < a.length) i++;
        break;
      }
      const [child, ni] = parseValue(a, i);
      if (child === null) break;
      i = ni;
      if (last === null) n.kid = child; else last.sib = child;
      last = child;
    }
    return [n, i];
  }
  if (c === 34) { const ni = scanString(a, i); return [{ tag: 2, val: ni - i, kid: null, sib: null }, ni]; }
  if (isNum(c)) {
    let j = i;
    while (j < a.length && isNum(a[j])) j++;
    return [{ tag: 1, val: j - i, kid: null, sib: null }, j];
  }
  if (isAlpha(c)) {
    let j = i;
    while (j < a.length && isAlpha(a[j])) j++;
    return [{ tag: 3, val: j - i, kid: null, sib: null }, j];
  }
  return [null, i + 1];
}

function walkRec(n, d) {
  if (n === null) return [0, 0];
  let seen = 1, acc = n.tag * d;
  for (let c = n.kid; c !== null; c = c.sib) {
    const [s, a] = walkRec(c, d + 1);
    seen += s; acc += a;
  }
  return [seen, acc];
}

export function treeRec(a) {
  const [root] = parseValue(a, skip(a, 0));
  const [seen, acc] = walkRec(root, 1);
  return seen * 1000 + acc;
}

// -------------------------------------------------------------------- flat

export function treeFlat(a) {
  const nodes = new Array(4 * NMAX).fill(0);
  const stk = new Array(2 * DMAX).fill(0);
  let i = 0, nn = 1, sp = 0;
  for (;;) {
    if (i >= a.length || nn >= NMAX || sp >= DMAX) break;
    const c = a[i];
    if (isSkip(c)) { i++; continue; }
    if (c === 123 || c === 91) {
      nodes[4 * nn] = c === 123 ? 5 : 4;
      nodes[4 * nn + 1] = 0;
      link(nodes, stk, sp, nn);
      if (sp >= 1) stk[2 * (sp - 1) + 1] = nn;
      stk[2 * sp] = nn;
      stk[2 * sp + 1] = 0;
      i++; sp++; nn++;
      continue;
    }
    if (c === 125 || c === 93) { i++; if (sp >= 1) sp--; continue; }
    if (c === 34 || isNum(c) || isAlpha(c)) {
      let tg = 2, ni = 0;
      if (c === 34) { ni = scanString(a, i); }
      else if (isNum(c)) { tg = 1; let j = i; while (j < a.length && isNum(a[j])) j++; ni = j; }
      else { tg = 3; let j = i; while (j < a.length && isAlpha(a[j])) j++; ni = j; }
      nodes[4 * nn] = tg;
      nodes[4 * nn + 1] = ni - i;
      link(nodes, stk, sp, nn);
      if (sp >= 1) stk[2 * (sp - 1) + 1] = nn;
      i = ni; nn++;
      continue;
    }
    i++;
  }
  return walkFlat(nodes);
}

function link(nodes, stk, sp, k) {
  if (sp < 1) return;
  const lc = stk[2 * (sp - 1) + 1];
  if (lc === 0) nodes[4 * stk[2 * (sp - 1)] + 2] = k;
  else nodes[4 * lc + 3] = k;
}

function walkFlat(nodes) {
  const wl = new Array(2 * NMAX).fill(0);
  wl[0] = 1; wl[1] = 1;
  let sp = 1, seen = 0, acc = 0, steps = 0;
  while (sp >= 1 && steps < 2 * NMAX) {
    const n = wl[2 * (sp - 1)], d = wl[2 * (sp - 1) + 1];
    const sb = nodes[4 * n + 3], kd = nodes[4 * n + 2];
    sp--;
    if (sb !== 0) { wl[2 * sp] = sb; wl[2 * sp + 1] = d; sp++; }
    if (kd !== 0) { wl[2 * sp] = kd; wl[2 * sp + 1] = d + 1; sp++; }
    seen++; acc += nodes[4 * n] * d; steps++;
  }
  return seen * 1000 + acc;
}

// ------------------------------------------------------------------- input

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
const doc = bytesOf(makeDoc(20));

function check() {
  let bad = 0;
  for (const n of [0, 1, 2, 5, 20]) {
    const a = bytesOf(makeDoc(n));
    const want = treeRec(a);
    for (const [nm, got] of [["flat", treeFlat(a)], ["NATIVE", nativeMeasure(a)]]) {
      if (got !== want) { console.log(`records=${n} ${nm}: ${got} != ${want}`); bad++; }
    }
  }
  for (const s of ["[1,2]", '{"a":1}', "[[1],2]", '{"a":[1,2],"b":true}',
                   "[]", "{}", "[[[[1]]]]", '["a\\"b"]']) {
    const a = bytesOf(s);
    const want = treeRec(a);
    for (const [nm, got] of [["flat", treeFlat(a)], ["NATIVE", nativeMeasure(a)]]) {
      if (got !== want) { console.log(`${JSON.stringify(s)} ${nm}: ${got} != ${want}`); bad++; }
    }
  }
  console.log(bad === 0 ? "all three agree" : `${bad} disagreement(s)`);
  if (bad) process.exit(1);
}

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
  console.log(`${name.padEnd(32)} ${ts[ts.length >> 1].toFixed(1).padStart(12)} ns/op`);
}

const CASES = {
  rec: ["T  tree recursive     hand", () => treeRec(doc), 2000],
  flat: ["T  tree flat          hand", () => treeFlat(doc), 2000],
  native: ["T  tree flat          NATIVE", () => nativeMeasure(doc), 2000],
};

const which = process.argv[2];
if (which === "--check" || which === undefined) check();
else {
  const c = CASES[which];
  if (!c) { console.error(`no such case: ${which}`); process.exit(1); }
  bench(c[0], c[1], c[2]);
}
