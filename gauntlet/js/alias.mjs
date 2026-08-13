import { makeVec } from "./gauntlet.mjs";

function smooth(dst, src) {
  for (let i = 1; i < src.length - 1; i++)
    dst[i] = (src[i - 1] + src[i] + src[i + 1]) / 3;
}
function smoothNoAlias(dst, src) {
  let a = src[0], b = src[1];
  for (let i = 1; i < src.length - 1; i++) {
    const c = src[i + 1];
    dst[i] = (a + b + c) / 3;
    a = b; b = c;
  }
}
function bench(name, fn, iters) {
  for (let i = 0; i < iters; i++) fn();
  const ts = [];
  for (let r = 0; r < 7; r++) {
    const t0 = process.hrtime.bigint();
    for (let i = 0; i < iters; i++) fn();
    ts.push(Number(process.hrtime.bigint() - t0) / iters);
  }
  ts.sort((a, b) => a - b);
  console.log(`${name.padEnd(30)} ${ts[3].toFixed(0).padStart(10)} ns/op`);
}
const N = 1 << 16;
const src = makeVec(N, 7);
const dst = new Float64Array(N);
bench("smooth disjoint", () => smooth(dst, src), 200);
bench("smoothNoAlias disjoint", () => smoothNoAlias(dst, src), 200);
bench("smooth IN-PLACE (aliased)", () => smooth(src, src), 200);
bench("smoothFresh (allocates)", () => { const d = new Float64Array(N); smooth(d, src); }, 200);
