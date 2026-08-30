// Karatsuba without recursion, in place, on JavaScript.
//
// The port of gauntlet/go/karatsuba2.go. Two of the three children are
// SUBRANGES of the parent, so a node is an offset and a length into one arena
// rather than a buffer, and the tree is a flat descriptor table filled by a
// loop. Nothing recurses and nothing is pushed.
//
// BASE 2^15, and the reason is bigarith-2026-08-28 §5a: the constraint on V8 is
// not 2^53 but 2^31, because leaving int32 costs a cliff. The schoolbook
// accumulation here is `a[i]*b[j] + out[i+j] + carry`; at 2^15 the product is
// under 2^30 and the whole sum stays inside int32. At 2^16 the product alone is
// 2^32.
//
//   node karatsuba.mjs check
//   node karatsuba.mjs <bits> <D> <iters>      one variant per process

const B = 15, M = 0x7fff;

function addAt(dst, dOff, dLen, src, sOff, sLen, off) {
  let carry = 0, i = 0;
  for (; i < sLen; i++) {
    const t = dst[dOff + off + i] + src[sOff + i] + carry;
    dst[dOff + off + i] = t & M;
    carry = t >>> B;
  }
  for (; carry !== 0 && off + i < dLen; i++) {
    const t = dst[dOff + off + i] + carry;
    dst[dOff + off + i] = t & M;
    carry = t >>> B;
  }
}

function subAt(dst, dOff, dLen, src, sOff, sLen, off) {
  let borrow = 0, i = 0;
  for (; i < sLen; i++) {
    const t = dst[dOff + off + i] - src[sOff + i] - borrow;
    if (t < 0) { dst[dOff + off + i] = t + 32768; borrow = 1; }
    else { dst[dOff + off + i] = t; borrow = 0; }
  }
  for (; borrow !== 0 && off + i < dLen; i++) {
    const t = dst[dOff + off + i] - borrow;
    if (t < 0) { dst[dOff + off + i] = t + 32768; borrow = 1; }
    else { dst[dOff + off + i] = t; borrow = 0; }
  }
}

function mulAt(ar, ao, bo, n, out, po, oLen) {
  out.fill(0, po, po + oLen);
  for (let i = 0; i < n; i++) {
    let carry = 0;
    const ai = ar[ao + i];
    for (let j = 0; j < n; j++) {
      const t = ai * ar[bo + j] + out[po + i + j] + carry;
      out[po + i + j] = t & M;
      carry = t >>> B;
    }
    out[po + i + n] = carry;
  }
}

const pow3 = (k) => { let p = 1; for (let i = 0; i < k; i++) p *= 3; return p; };

export class KWork {
  constructor(n, D) {
    this.n = n; this.D = D;
    const lenOf = new Int32Array(D + 1);
    lenOf[0] = n;
    for (let L = 0; L < D; L++) lenOf[L + 1] = (lenOf[L] - (lenOf[L] >> 1)) + 1;
    this.lenOf = lenOf;

    const baseIdx = new Int32Array(D + 2);
    let acc = 0, p = 1;
    for (let L = 0; L <= D; L++) { baseIdx[L] = acc; acc += p; p *= 3; }
    baseIdx[D + 1] = acc;
    this.baseIdx = baseIdx;

    let ar = 2 * n;
    p = 1;
    for (let L = 0; L < D; L++) { ar += p * 2 * lenOf[L + 1]; p *= 3; }
    this.arena = new Int32Array(ar);

    this.aOff = new Int32Array(acc);
    this.bOff = new Int32Array(acc);
    this.ln = new Int32Array(acc);
    this.pOff = new Int32Array(acc);

    // Product sizes bottom-up and exact: a parent must reach 2h + a child's.
    const prodOf = new Int32Array(D + 1);
    prodOf[D] = 2 * lenOf[D];
    for (let L = D - 1; L >= 0; L--) prodOf[L] = 2 * (lenOf[L] >> 1) + prodOf[L + 1];
    this.prodOf = prodOf;

    let tot = 0;
    p = 1;
    for (let L = 0; L <= D; L++) {
      for (let k = 0; k < p; k++) { this.pOff[baseIdx[L] + k] = tot; tot += prodOf[L]; }
      p *= 3;
    }
    this.prod = new Int32Array(tot);
  }

  mul(a, b) {
    const { n, D, arena: ar, prod, aOff, bOff, ln, pOff, lenOf, prodOf, baseIdx } = this;
    ar.set(a, 0);
    ar.set(b, n);
    aOff[0] = 0; bOff[0] = n; ln[0] = n;

    let free = 2 * n;
    for (let L = 0; L < D; L++) {
      const base = baseIdx[L], cbase = baseIdx[L + 1], cl = lenOf[L + 1];
      for (let k = 0, p = pow3(L); k < p; k++) {
        const id = base + k, ao = aOff[id], bo = bOff[id], l = ln[id], h = l >> 1;
        const c0 = cbase + 3 * k;
        aOff[c0] = ao; bOff[c0] = bo; ln[c0] = h;
        aOff[c0 + 1] = ao + h; bOff[c0 + 1] = bo + h; ln[c0 + 1] = l - h;
        const as = free, bs = free + cl;
        free += 2 * cl;
        aOff[c0 + 2] = as; bOff[c0 + 2] = bs; ln[c0 + 2] = cl;
        ar.fill(0, as, as + 2 * cl);
        ar.copyWithin(as, ao + h, ao + l);
        addAt(ar, as, cl, ar, ao, h, 0);
        ar.copyWithin(bs, bo + h, bo + l);
        addAt(ar, bs, cl, ar, bo, h, 0);
      }
    }

    {
      const base = baseIdx[D];
      for (let k = 0, p = pow3(D); k < p; k++) {
        const id = base + k;
        mulAt(ar, aOff[id], bOff[id], ln[id], prod, pOff[id], prodOf[D]);
      }
    }

    for (let L = D - 1; L >= 0; L--) {
      const b0 = baseIdx[L], cbase = baseIdx[L + 1], csz = prodOf[L + 1], sz = prodOf[L];
      for (let k = 0, p = pow3(L); k < p; k++) {
        const id = b0 + k, h = ln[id] >> 1, po = pOff[id];
        prod.fill(0, po, po + sz);
        const c0 = pOff[cbase + 3 * k];
        addAt(prod, po, sz, prod, c0, csz, 0);
        addAt(prod, po, sz, prod, c0 + csz, csz, 2 * h);
        addAt(prod, po, sz, prod, c0 + 2 * csz, csz, h);
        subAt(prod, po, sz, prod, c0, csz, h);
        subAt(prod, po, sz, prod, c0 + csz, csz, h);
      }
    }
    return prod;
  }
}

// ----------------------------------------------------------------- harness

function limbsOf(n, seed) {
  const out = new Int32Array(n);
  let x = seed >>> 0;
  for (let i = 0; i < n; i++) { x = (Math.imul(x, 1103515245) + 12345) >>> 0; out[i] = (x >>> 8) & M; }
  out[n - 1] |= 1 << (B - 1);
  return out;
}

function toBig(l, off, n) {
  let out = 0n;
  const W = BigInt(B);
  for (let i = n - 1; i >= 0; i--) out = (out << W) | BigInt(l[off + i]);
  return out;
}

function check() {
  let bad = 0;
  for (const n of [16, 32, 64, 256, 1024]) {
    const a = limbsOf(n, 12345), b = limbsOf(n, 67890);
    const want = toBig(a, 0, n) * toBig(b, 0, n);
    for (let d = 0; d <= 5 && (n >> d) >= 4; d++) {
      const w = new KWork(n, d);
      if (toBig(w.mul(a, b), 0, 2 * n) !== want) { console.log(`FAIL n=${n} D=${d}`); bad++; }
    }
  }
  console.log(bad === 0 ? "ok — every depth agrees with BigInt" : `${bad} FAILURES`);
  return bad;
}

// TIME-BUDGETED, NOT COUNT-BUDGETED, and that is a correction rather than a
// preference. Measured with a fixed iteration count, V8's `BigInt` multiply
// came out at 5,900 ns/op at 20,000 iterations and 23,000 at 2,000 — a 4x
// spread on identical work, from the count alone. A ratio taken against a
// number like that is not a result, and bigarith-2026-08-28 §8's "148x" was
// one. Running each side for the same wall-clock budget removes the axis.
function bench(fn, ms = 500) {
  let t = Date.now();
  while (Date.now() - t < ms) fn();
  const times = [];
  for (let r = 0; r < 5; r++) {
    let n = 0;
    const t0 = process.hrtime.bigint();
    let el;
    do { fn(); n++; el = Number(process.hrtime.bigint() - t0); } while (el < ms * 1e6);
    times.push(el / n);
  }
  times.sort((x, y) => x - y);
  return times[2];
}

// A SINK THE RESULT ESCAPES INTO. Without it V8 eliminates `x * y` entirely —
// the operands are loop-invariant and the product unused — and a 16,384-bit
// multiply "measured" 48 ns/op, which is the shape of a mistake rather than a
// result. The Karatsuba side never needed one: it writes into its workspace.
let sink = 0n;

const argv = process.argv.slice(2);
if (argv[0] === "check") process.exit(check() === 0 ? 0 : 1);

const bits = Number(argv[0]), D = argv[1];
const n = Math.ceil(bits / B);
const a = limbsOf(n, 12345), b = limbsOf(n, 67890);
let fn, label;
if (D === "bigint") {
  const x = toBig(a, 0, n), y = toBig(b, 0, n);
  fn = () => { sink = x * y; };
  label = "BigInt";
} else {
  const w = new KWork(n, Number(D));
  fn = () => w.mul(a, b);
  label = "Karatsuba D=" + D;
}
const ns = bench(fn);
// Read the sink so nothing above can be proven dead.
if (typeof sink === "bigint" && sink === 1n) console.log("unreachable");
console.log(`${String(bits).padStart(6)} bits  ${label.padEnd(16)} ${ns.toFixed(1)} ns/op`);
