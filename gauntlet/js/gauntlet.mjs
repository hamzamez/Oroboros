// Hand-written JavaScript reference implementations.
//
// Where the derivations made a claim about JS, this file carries both forms:
//
//   G1  dotTyped vs dotArray        — does Float64Array beat a plain Array?
//   G2  centroidAoS vs centroidSoA  — the g2 claim that array-of-objects loses
//   G3  sumMono vs sumFold          — what does a callback parameter cost?
//   G4  wordCountMap                — must use Map, not a hash table of ours
//   G6  buildOps / makeScaler       — escaping closures

// ---------------------------------------------------------------- G1

export function dotTyped(xs, ys) {
  let acc = 0.0;
  for (let i = 0; i < xs.length; i++) acc += xs[i] * ys[i];
  return acc;
}

export function dotArray(xs, ys) {
  let acc = 0.0;
  for (let i = 0; i < xs.length; i++) acc += xs[i] * ys[i];
  return acc;
}

export function dotUnordered4(xs, ys) {
  let a0 = 0.0, a1 = 0.0, a2 = 0.0, a3 = 0.0;
  const n = xs.length - (xs.length % 4);
  for (let i = 0; i < n; i += 4) {
    a0 += xs[i] * ys[i];
    a1 += xs[i + 1] * ys[i + 1];
    a2 += xs[i + 2] * ys[i + 2];
    a3 += xs[i + 3] * ys[i + 3];
  }
  let acc = (a0 + a1) + (a2 + a3);
  for (let i = n; i < xs.length; i++) acc += xs[i] * ys[i];
  return acc;
}

// ---------------------------------------------------------------- G2

// Array of {x, y} objects: what a naive AoS lowering would emit.
export function centroidAoS(ps) {
  let accX = 0.0, accY = 0.0;
  for (let i = 0; i < ps.length; i++) { accX += ps[i].x; accY += ps[i].y; }
  const n = ps.length;
  return { x: accX / n, y: accY / n };
}

// Parallel Float64Arrays: what fast hand-written JS does, and what the g2
// derivation says a struct-of-arrays representation would emit.
export function centroidSoA(px, py) {
  let accX = 0.0, accY = 0.0;
  for (let i = 0; i < px.length; i++) { accX += px[i]; accY += py[i]; }
  const n = px.length;
  return { x: accX / n, y: accY / n };
}

export function boundsSoA(px, py) {
  let loX = px[0], loY = py[0], hiX = px[0], hiY = py[0];
  for (let i = 1; i < px.length; i++) {
    const x = px[i], y = py[i];
    if (x < loX) loX = x;
    if (y < loY) loY = y;
    if (x > hiX) hiX = x;
    if (y > hiY) hiY = y;
  }
  return { loX, loY, hiX, hiY };
}

// ---------------------------------------------------------------- G3

export function sumMono(xs) {
  let acc = 0.0;
  for (let i = 0; i < xs.length; i++) acc += xs[i];
  return acc;
}

export function fold(xs, init, step) {
  let acc = init;
  for (let i = 0; i < xs.length; i++) acc = step(acc, xs[i]);
  return acc;
}

export function sumFold(xs) {
  return fold(xs, 0.0, (a, x) => a + x);
}

export function countPositiveMono(xs) {
  let acc = 0;
  for (let i = 0; i < xs.length; i++) if (xs[i] > 0) acc++;
  return acc;
}

export function countPositiveFold(xs) {
  return fold(xs, 0, (n, x) => (x > 0 ? n + 1 : n));
}

// ---------------------------------------------------------------- G4

export function wordCountMap(text) {
  const counts = new Map();
  const ws = text.split(" ");
  for (let i = 0; i < ws.length; i++) {
    const w = ws[i];
    counts.set(w, (counts.get(w) ?? 0) + 1);
  }
  return counts;
}

// A plain object as the dictionary, for comparison.
export function wordCountObject(text) {
  const counts = Object.create(null);
  const ws = text.split(" ");
  for (let i = 0; i < ws.length; i++) {
    const w = ws[i];
    counts[w] = (counts[w] ?? 0) + 1;
  }
  return counts;
}

// ---------------------------------------------------------------- G6

export function buildOps() {
  return [(v) => (v + 1) | 0, (v) => (v * 2) | 0, (v) => -v | 0];
}

export function runOp(ops, k, x) {
  return ops[k](x);
}

export function makeScaler(f) {
  return (v) => (v * f) | 0;
}

// ---------------------------------------------------------------- inputs

// Mulberry32: small, deterministic, identical across languages.
export function rng(seed) {
  let a = seed >>> 0;
  return function () {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

export function makeVec(n, seed) {
  const r = rng(seed);
  const xs = new Float64Array(n);
  for (let i = 0; i < n; i++) xs[i] = r() * 2 - 1;
  return xs;
}

export function makeVecArray(n, seed) {
  const r = rng(seed);
  const xs = new Array(n);
  for (let i = 0; i < n; i++) xs[i] = r() * 2 - 1;
  return xs;
}

export function makePointsAoS(n, seed) {
  const r = rng(seed);
  const ps = new Array(n);
  for (let i = 0; i < n; i++) ps[i] = { x: r() * 2 - 1, y: r() * 2 - 1 };
  return ps;
}

export function makeText(n, seed) {
  const r = rng(seed);
  const vocab = new Array(500);
  for (let i = 0; i < 500; i++) {
    const len = 3 + Math.floor(r() * 6);
    let s = "";
    for (let j = 0; j < len; j++) s += String.fromCharCode(97 + Math.floor(r() * 26));
    vocab[i] = s;
  }
  const out = new Array(n);
  for (let i = 0; i < n; i++) out[i] = vocab[Math.floor(r() * 500)];
  return out.join(" ");
}
