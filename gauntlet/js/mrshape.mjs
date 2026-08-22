// Which representation should an emitted JS function use for several results?
//
// product-2026-08-19 measured object 1.11x / array 1.32x for a create-and-consume
// pair INSIDE one function. That is not this shape: here the value crosses an
// exported boundary and a hand-written caller takes it apart.
const N = 1000;
class Pair { constructor(a, b) { this.f0 = a; this.f1 = b; } }

function retArray(a, b)  { return [a / b | 0, a % b]; }
function retObject(a, b) { return { f0: a / b | 0, f1: a % b }; }
function retClass(a, b)  { return new Pair(a / b | 0, a % b); }

let sq = 0, sr = 0;
const cases = {
  array:  () => { for (let i = 0; i < N; i++) { const p = retArray(i | 1, 7);  sq = p[0];  sr = p[1]; } },
  arrayd: () => { for (let i = 0; i < N; i++) { const [q, r] = retArray(i | 1, 7); sq = q; sr = r; } },
  object: () => { for (let i = 0; i < N; i++) { const p = retObject(i | 1, 7); sq = p.f0; sr = p.f1; } },
  objectd:() => { for (let i = 0; i < N; i++) { const {f0, f1} = retObject(i | 1, 7); sq = f0; sr = f1; } },
  klass:  () => { for (let i = 0; i < N; i++) { const p = retClass(i | 1, 7);  sq = p.f0; sr = p.f1; } },
  none:   () => { for (let i = 0; i < N; i++) { sq = (i | 1) / 7 | 0; sr = (i | 1) % 7; } },
};

function bench(name, fn, iters) {
  const w0 = process.hrtime.bigint();
  for (let i = 0; i < 20000; i++) { fn(); if (i >= 1000 && Number(process.hrtime.bigint() - w0) > 2e9) break; }
  const ts = [];
  for (let r = 0; r < 7; r++) {
    const t0 = process.hrtime.bigint();
    for (let i = 0; i < iters; i++) fn();
    ts.push(Number(process.hrtime.bigint() - t0) / iters);
  }
  ts.sort((a, b) => a - b);
  console.log(`${name.padEnd(10)} ${ts[ts.length >> 1].toFixed(1).padStart(10)} ns/op`);
}
const w = process.argv[2];
if (w === "--list") console.log(Object.keys(cases).join("\n"));
else bench(w, cases[w], 2000);
