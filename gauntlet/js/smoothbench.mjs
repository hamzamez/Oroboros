import { genSmooth } from "./generated_smooth.mjs";

const N = 65536, PASSES = 8;
const base = new Array(N);
for (let i = 0; i < N; i++) base[i] = (i % 17) * 0.25;

// Hand-written, caller owns dst — the shape we lose to on Go.
function smoothInto(dst, src) {
  const n = src.length - 2;
  for (let i = 0; i < n; i++) dst[i] = (src[i] + src[i + 1] + src[i + 2]) / 3;
}

function bench(name, f) {
  for (let i = 0; i < 20; i++) f();
  const t0 = process.hrtime.bigint();
  for (let i = 0; i < 60; i++) f();
  const t1 = process.hrtime.bigint();
  console.log(name.padEnd(30), (Number(t1 - t0) / 60 / 1000).toFixed(1).padStart(9), "us/op");
}

const dstA = new Array(N), dstB = new Array(N);
bench("SmoothInto repeated", () => {
  let src = base, dst = dstA, spare = dstB;
  for (let p = 0; p < PASSES; p++) { smoothInto(dst, src); [src, dst] = [dst, src === base ? spare : src]; }
  return src;
});
bench("genSmooth repeated", () => {
  let v = base;
  for (let p = 0; p < PASSES; p++) v = genSmooth(v);
  return v;
});
