// COUNT-THEN-BUILD AGAINST push, on V8.
//
// growth.md's third measurement. The array-language answer to `filter` is two
// passes — count the survivors, allocate exactly, fill — which needs no growth.
// `push` needs growth. growth.md PREDICTED two passes would win on Go and
// windows and LOSE on JavaScript; this is the test of that.
//
//   node growfilter.mjs check | <variant>

function input(n) {
  const out = new Float64Array(n);
  let x = 12345 >>> 0;
  for (let i = 0; i < n; i++) { x = (Math.imul(x, 1103515245) + 12345) >>> 0; out[i] = x >>> 3; }
  return out;
}

export function filterPush(a) {
  const out = [];
  for (let i = 0; i < a.length; i++) if ((a[i] & 1) === 0) out.push(a[i]);
  return out;
}

export function filterPushTyped(a) {
  const out = new Float64Array(a.length);
  let k = 0;
  for (let i = 0; i < a.length; i++) if ((a[i] & 1) === 0) out[k++] = a[i];
  return out.subarray(0, k);
}

export function filterCountBuild(a) {
  let n = 0;
  for (let i = 0; i < a.length; i++) if ((a[i] & 1) === 0) n++;
  const out = new Float64Array(n);
  let k = 0;
  for (let i = 0; i < a.length; i++) if ((a[i] & 1) === 0) out[k++] = a[i];
  return out;
}

// LIKE-FOR-LIKE with filterPush: both produce a plain Array, so the comparison
// is growth against two passes and not Array against Float64Array — which the
// first run conflated, and which V8 cares about a lot (a packed Array of small
// integers is Smi-backed; a Float64Array is not).
export function filterCountBuildArray(a) {
  let n = 0;
  for (let i = 0; i < a.length; i++) if ((a[i] & 1) === 0) n++;
  const out = new Array(n);
  let k = 0;
  for (let i = 0; i < a.length; i++) if ((a[i] & 1) === 0) out[k++] = a[i];
  return out;
}

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
const a = input(N);

if (process.argv[2] === "check") {
  const x = filterPush(a), y = filterPushTyped(a), z = filterCountBuild(a), w = filterCountBuildArray(a);
  let bad = x.length !== y.length || x.length !== z.length || x.length !== w.length ? 1 : 0;
  for (let i = 0; i < x.length && !bad; i++) if (x[i] !== y[i] || x[i] !== z[i] || x[i] !== w[i]) bad = 1;
  console.log(bad ? "FAIL" : `ok — all forms agree, ${x.length} of ${N} survive`);
  process.exit(bad);
}

const v = { push: () => filterPush(a), "push-typed": () => filterPushTyped(a),
            "count-build": () => filterCountBuild(a),
            "count-build-array": () => filterCountBuildArray(a) }[process.argv[2]];
if (!v) { console.error("unknown"); process.exit(2); }
const ns = bench(v);
if (sink === undefined) console.log("unreachable");
console.log(`${process.argv[2].padEnd(13)} ${ns.toFixed(0).padStart(9)} ns/op`);
