// THE GAUNTLET'S PAIRS, INTERLEAVED (CLAUDE.md's measurements' method,
// jsontok-2026-11-19 §3 (b)): a process slows with its age, so the generated
// form and its reference are timed back to back each round, the order
// alternating (ABBA cancels a slow drift's first-order term), and the pair's
// figure is the median of the rounds' ratios.
//
// Each form gets a timing loop of its own, compiled from text, so V8 sees one
// target at each call site: one shared loop over two functions is the
// megamorphic call site of CLAUDE.md's benchmark-method errors.

const LOOP = `
  const t0 = process.hrtime.bigint();
  for (let i = 0; i < iters; i++) fn();
  return Number(process.hrtime.bigint() - t0) / iters;`;

// Each source is distinct, so V8's compilation cache cannot hand the two forms
// one function and one feedback vector.
let timers = 0;
function timer() {
  return new Function("fn", "iters", "process", `// timer ${timers++}` + LOOP);
}

function median(xs) {
  const s = [...xs].sort((a, b) => a - b);
  return s.length % 2 ? s[s.length >> 1] : (s[s.length / 2 - 1] + s[s.length / 2]) / 2;
}

// interleave(gen, hand, opts): gen and hand are [label, fn]. A batch is sized so
// the reference's lasts about opts.batchMs, and both run that many calls.
export function interleave(gen, hand, { rounds = 7, batchMs = 20 } = {}) {
  const tg = timer(), th = timer();
  // warm both to V8's optimizing tier: at least 1000 calls, at most 2 s each
  for (const [f, t] of [[gen[1], tg], [hand[1], th]]) {
    const w0 = process.hrtime.bigint();
    for (let i = 0; i < 20; i++) {
      t(f, 1000, process);
      if (Number(process.hrtime.bigint() - w0) > 2e9) break;
    }
  }
  const per = th(hand[1], 1000, process);
  const n = Math.max(1, Math.round((batchMs * 1e6) / per));
  const gs = [], hs = [], rs = [];
  for (let r = 0; r < rounds; r++) {
    let g, h;
    if (r % 2 === 0) { g = tg(gen[1], n, process); h = th(hand[1], n, process); }
    else { h = th(hand[1], n, process); g = tg(gen[1], n, process); }
    gs.push(g); hs.push(h); rs.push(g / h);
  }
  const s = [...rs].sort((a, b) => a - b);
  return { n, genNs: median(gs), handNs: median(hs), ratio: median(rs), lo: s[0], hi: s[s.length - 1], ratios: rs };
}

export function printPair(genName, handName, r) {
  console.log(`${genName.padEnd(22)} ${handName.padEnd(22)} ${String(r.n).padStart(8)} ` +
    `${r.genNs.toFixed(1).padStart(12)} ${r.handNs.toFixed(1).padStart(12)} ` +
    `${r.ratio.toFixed(3).padStart(7)} ${r.lo.toFixed(3)}–${r.hi.toFixed(3)} pair`);
}

// runPairs reads `--pair gen hand [gen hand …] [--rounds k]` from argv after
// its index, and measures each pair from cases, {name: [label, fn, iters]}.
export function runPairs(cases, argv) {
  let rounds = 7;
  const names = [];
  for (let i = 0; i < argv.length; i++) {
    if (argv[i] === "--rounds") rounds = Number(argv[++i]);
    else names.push(argv[i]);
  }
  if (names.length % 2 !== 0) { console.error("--pair takes gen hand names in pairs"); process.exit(1); }
  for (let i = 0; i < names.length; i += 2) {
    const g = cases[names[i]], h = cases[names[i + 1]];
    if (!g || !h) { console.error(`no such case: ${!g ? names[i] : names[i + 1]}`); process.exit(1); }
    printPair(names[i], names[i + 1], interleave(g, h, { rounds }));
  }
}

// THE HARNESS CAN FAIL: `node pairs.mjs` checks that twice the work reads about
// 2 and a form against itself about 1.
import { pathToFileURL } from "node:url";
if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  const a = new Float64Array(1024).map((_, i) => i * 0.5);
  let sink = 0;
  const one = () => { let s = 0; for (let i = 0; i < a.length; i++) s += a[i] * a[i]; sink += s; };
  // twice the work is the same call twice: a second loop of other arithmetic
  // read 1.29 under V8, whose cost per loop is not a count of operations
  const two = () => { one(); one(); };
  const r2 = interleave(["two", two], ["one", one], { rounds: 5, batchMs: 10 });
  const r1 = interleave(["one", one], ["one'", one], { rounds: 5, batchMs: 10 });
  // two distinct closures doing the same work, in both orders: the first
  // timed form read 1.7x the second when the two timers shared one source
  // (pairs-2026-11-19), which a form against itself cannot show
  const b = new Float64Array(1 << 16).map((_, i) => i * 0.25);
  const c = new Float64Array(1 << 16).map((_, i) => i * 0.5);
  const dot = (x, y) => { let s = 0; for (let i = 0; i < x.length; i++) s += x[i] * y[i]; return s; };
  const p = () => { sink += dot(b, c); };
  const q = () => { sink += dot(b, c); };
  const rpq = interleave(["p", p], ["q", q], { rounds: 5, batchMs: 10 });
  const ok = r2.ratio > 1.5 && r2.ratio < 2.6 && r1.ratio > 0.85 && r1.ratio < 1.18 &&
    rpq.ratio > 0.85 && rpq.ratio < 1.18;
  console.log(`twice the work ${r2.ratio.toFixed(3)}, itself ${r1.ratio.toFixed(3)}, ` +
    `its twin ${rpq.ratio.toFixed(3)}: ${ok ? "ok" : "FAIL"} (${sink > 0})`);
  process.exit(ok ? 0 : 1);
}
