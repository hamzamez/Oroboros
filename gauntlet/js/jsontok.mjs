// A JSON tokeniser on JavaScript: emitted against hand-written.
//
// The first BRANCHY program in the gauntlet. Everything measured on this host
// before is a numeric loop over a Float64Array, which is the shape V8 is best
// at and the shape our emitter has been tuned against. A tokeniser is a
// data-dependent switch per byte with unpredictable branches.
//
// FOUR FORMS, because "what a person writes" is genuinely ambiguous here and
// the memory shape is the thing most likely to decide the answer:
//
//   string    src.charCodeAt(i)     — what a person actually writes
//   u8        Uint8Array            — the fast path a person reaches for
//   array     plain Array of ints   — OUR memory shape
//   NATIVE    emitted from the .oro — also a plain Array
//
// Only `array` and `NATIVE` are a like-for-like comparison; the other two are
// what the host can do at its best, which is the number that matters for
// requirement 5 in the end.
//
//   node jsontok.mjs --check      correctness only
//   node jsontok.mjs --list       the case names
//   node jsontok.mjs <case>       run one case
//
// ONE PROCESS PER COMPARISON. V8 carries optimization state, inline caches and
// GC pressure across benchmarks in one process — native-gauntlet-2026-08-20 §9
// measured the same function at 32,902 ns and then 236,497 ns on consecutive
// runs in one process. Each case here is meant to be its own `node`.
//
// NAMED imports on both sides. Reaching a function through a module namespace
// object costs 1.66x for the property load, which would make the generated side
// look faster for no reason (native-js-2026-08-20).

import { nativeTokens } from "./gen_jsontok_native.mjs";

const CAP = 32;

const isSpace = (c) => c === 32 || c === 9 || c === 10 || c === 13;
const isDigit = (c) => c >= 48 && c <= 57;
const isAlpha = (c) => c >= 97 && c <= 122;
const isNum = (c) =>
  isDigit(c) || c === 45 || c === 43 || c === 46 || c === 101 || c === 69;
const isOpen = (c) => c === 123 || c === 91;
const isClose = (c) => c === 125 || c === 93;
const isPunct = (c) => c === 58 || c === 44;

// THREE SEPARATE BODIES, NOT ONE PARAMETERISED BY A CLOSURE.
//
// The first version of this file had one `tokenize(len, at)` and passed
// `(i) => s.charCodeAt(i)`, which is an indirect call PER BYTE. It measured the
// generated code at 0.68x hand-written, and that number was measuring the
// closure. The same class of error as native-js-2026-08-20's `g.dotTyped`:
// 1.66x for a property load that had nothing to do with the code under test.
//
// So the access is written inline in each, and the three are otherwise
// identical. Duplication is the correct answer when the thing being measured is
// how the host compiles the access.

function tokStringBody(s) {
  const stk = new Int32Array(CAP);
  const len = s.length;
  let i = 0, nt = 0, sp = 0, mx = 0, ok = 1;
  for (;;) {
    if (i >= len) return nt * 1000 + mx * 10 + (sp === 0 ? ok : 0);
    if (sp >= CAP) return nt * 1000;
    const c = s.charCodeAt(i);
    if (c === 32 || c === 9 || c === 10 || c === 13) { i++; continue; }
    if (c === 123 || c === 91) {
      stk[sp] = c === 123 ? 125 : 93;
      i++; nt++; sp++;
      if (sp > mx) mx = sp;
      continue;
    }
    if (c === 125 || c === 93) {
      i++; nt++;
      if (sp < 1) { ok = 0; } else { sp--; if (stk[sp] !== c) ok = 0; }
      continue;
    }
    if (c === 58 || c === 44) { i++; nt++; continue; }
    if (c === 34) {
      let j = i + 1;
      for (;;) {
        if (j >= len) break;
        const d = s.charCodeAt(j);
        if (d === 92) { j += 2; continue; }
        if (d === 34) { j++; break; }
        j++;
      }
      i = j; nt++;
      continue;
    }
    if ((c >= 48 && c <= 57) || c === 45 || c === 43 || c === 46 || c === 101 || c === 69) {
      let j = i;
      for (;;) {
        if (j >= len) break;
        const d = s.charCodeAt(j);
        if (!((d >= 48 && d <= 57) || d === 45 || d === 43 || d === 46 || d === 101 || d === 69)) break;
        j++;
      }
      i = j; nt++;
      continue;
    }
    if (c >= 97 && c <= 122) {
      let j = i;
      while (j < len) { const d = s.charCodeAt(j); if (d < 97 || d > 122) break; j++; }
      i = j; nt++;
      continue;
    }
    i++; ok = 0;
  }
}

function tokU8Body(a) {
  const stk = new Int32Array(CAP);
  const len = a.length;
  let i = 0, nt = 0, sp = 0, mx = 0, ok = 1;
  for (;;) {
    if (i >= len) return nt * 1000 + mx * 10 + (sp === 0 ? ok : 0);
    if (sp >= CAP) return nt * 1000;
    const c = a[i];
    if (c === 32 || c === 9 || c === 10 || c === 13) { i++; continue; }
    if (c === 123 || c === 91) {
      stk[sp] = c === 123 ? 125 : 93;
      i++; nt++; sp++;
      if (sp > mx) mx = sp;
      continue;
    }
    if (c === 125 || c === 93) {
      i++; nt++;
      if (sp < 1) { ok = 0; } else { sp--; if (stk[sp] !== c) ok = 0; }
      continue;
    }
    if (c === 58 || c === 44) { i++; nt++; continue; }
    if (c === 34) {
      let j = i + 1;
      for (;;) {
        if (j >= len) break;
        const d = a[j];
        if (d === 92) { j += 2; continue; }
        if (d === 34) { j++; break; }
        j++;
      }
      i = j; nt++;
      continue;
    }
    if ((c >= 48 && c <= 57) || c === 45 || c === 43 || c === 46 || c === 101 || c === 69) {
      let j = i;
      for (;;) {
        if (j >= len) break;
        const d = a[j];
        if (!((d >= 48 && d <= 57) || d === 45 || d === 43 || d === 46 || d === 101 || d === 69)) break;
        j++;
      }
      i = j; nt++;
      continue;
    }
    if (c >= 97 && c <= 122) {
      let j = i;
      while (j < len) { const d = a[j]; if (d < 97 || d > 122) break; j++; }
      i = j; nt++;
      continue;
    }
    i++; ok = 0;
  }
}

// The SAME body again, spelled out. A Uint8Array and a plain Array are
// different receiver maps, and one function called with both goes polymorphic —
// which would penalise whichever form ran second for a reason that has nothing
// to do with tokenising.
function tokArrayBody(a) {
  const stk = new Int32Array(CAP);
  const len = a.length;
  let i = 0, nt = 0, sp = 0, mx = 0, ok = 1;
  for (;;) {
    if (i >= len) return nt * 1000 + mx * 10 + (sp === 0 ? ok : 0);
    if (sp >= CAP) return nt * 1000;
    const c = a[i];
    if (c === 32 || c === 9 || c === 10 || c === 13) { i++; continue; }
    if (c === 123 || c === 91) {
      stk[sp] = c === 123 ? 125 : 93;
      i++; nt++; sp++;
      if (sp > mx) mx = sp;
      continue;
    }
    if (c === 125 || c === 93) {
      i++; nt++;
      if (sp < 1) { ok = 0; } else { sp--; if (stk[sp] !== c) ok = 0; }
      continue;
    }
    if (c === 58 || c === 44) { i++; nt++; continue; }
    if (c === 34) {
      let j = i + 1;
      for (;;) {
        if (j >= len) break;
        const d = a[j];
        if (d === 92) { j += 2; continue; }
        if (d === 34) { j++; break; }
        j++;
      }
      i = j; nt++;
      continue;
    }
    if ((c >= 48 && c <= 57) || c === 45 || c === 43 || c === 46 || c === 101 || c === 69) {
      let j = i;
      for (;;) {
        if (j >= len) break;
        const d = a[j];
        if (!((d >= 48 && d <= 57) || d === 45 || d === 43 || d === 46 || d === 101 || d === 69)) break;
        j++;
      }
      i = j; nt++;
      continue;
    }
    if (c >= 97 && c <= 122) {
      let j = i;
      while (j < len) { const d = a[j]; if (d < 97 || d > 122) break; j++; }
      i = j; nt++;
      continue;
    }
    i++; ok = 0;
  }
}

export function tokString(s) { return tokStringBody(s); }
export function tokU8(a) { return tokU8Body(a); }
export function tokArray(a) { return tokArrayBody(a); }

// ------------------------------------------------------------------ input

export function makeDoc(records) {
  let s = '{"items":[';
  for (let r = 0; r < records; r++) {
    if (r > 0) s += ",";
    s += '{"id":1234,"name":"a b\\"c","tags":["x","y","z"],' +
      '"score":-12.5e3,"ok":true,"prev":null,' +
      '"meta":{"depth":2,"flag":false}}';
  }
  return s + "]}";
}

const docText = makeDoc(64);
const docU8 = new Uint8Array(docText.length);
for (let i = 0; i < docText.length; i++) docU8[i] = docText.charCodeAt(i);
const docArr = Array.from(docU8);

// ------------------------------------------------------------- correctness

function check() {
  let bad = 0;
  const cases = [0, 1, 3, 17];
  for (const n of cases) {
    const t = makeDoc(n);
    const a = Array.from(t, (ch) => ch.charCodeAt(0));
    const u = Uint8Array.from(a);
    const want = tokString(t);
    for (const [nm, got] of [["u8", tokU8(u)], ["array", tokArray(a)],
                             ["NATIVE", nativeTokens(a)]]) {
      if (got !== want) { console.log(`records=${n} ${nm}: ${got} != ${want}`); bad++; }
    }
  }
  // The malformed inputs matter more: that is where the four could differ.
  for (const s of ["]", '{"a":1', "[}", "{]", "", "[[[[1]]]]", ""]) {
    const a = Array.from(s, (ch) => ch.charCodeAt(0));
    const want = tokString(s);
    for (const [nm, got] of [["u8", tokU8(Uint8Array.from(a))], ["array", tokArray(a)],
                             ["NATIVE", nativeTokens(a)]]) {
      if (got !== want) { console.log(`${JSON.stringify(s)} ${nm}: ${got} != ${want}`); bad++; }
    }
  }
  console.log(bad === 0 ? "all four agree" : `${bad} disagreement(s)`);
  if (bad) process.exit(1);
}

// --------------------------------------------------------------- the bench

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
  console.log(`${name.padEnd(34)} ${ts[ts.length >> 1].toFixed(1).padStart(12)} ns/op`);
}

const CASES = {
  "string": ["J  tokenize string     hand", () => tokString(docText), 2000],
  "u8": ["J  tokenize Uint8Array hand", () => tokU8(docU8), 2000],
  "array": ["J  tokenize Array      hand", () => tokArray(docArr), 2000],
  "native": ["J  tokenize Array      NATIVE", () => nativeTokens(docArr), 2000],
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
