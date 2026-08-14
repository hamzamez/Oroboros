import { makeText } from "./gauntlet.mjs";
import { genWordCount } from "./generated_wc.mjs";

function wcRef(text) {
  const counts = Object.create(null);
  const ws = text.split(" ");
  for (let i = 0; i < ws.length; i++) counts[ws[i]] = (counts[ws[i]] ?? 0) + 1;
  return counts;
}

const text = makeText(2000, 5);
const a = genWordCount(text), b = wcRef(text);
if (Object.keys(a).length !== Object.keys(b).length) throw new Error("size mismatch");
for (const k of Object.keys(b)) if (a[k] !== b[k]) throw new Error("mismatch " + k);
console.log("correctness: generated agrees with hand-written\n");

function bench(name, fn, iters, runs = 5) {
  for (let i = 0; i < iters; i++) fn();
  const ts = [];
  for (let r = 0; r < runs; r++) {
    const t0 = process.hrtime.bigint();
    for (let i = 0; i < iters; i++) fn();
    ts.push(Number(process.hrtime.bigint() - t0) / iters);
  }
  ts.sort((x, y) => x - y);
  console.log(`${name.padEnd(24)} ${(ts[runs >> 1] / 1000).toFixed(1).padStart(12)} us/op`);
}
bench("hand-written", () => wcRef(text), 200);
bench("GENERATED", () => genWordCount(text), 3);
