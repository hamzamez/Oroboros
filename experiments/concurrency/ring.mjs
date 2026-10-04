// The ring on JavaScript: N processes, T tokens, M rounds each.
//   native    — an async function per process awaiting a promise mailbox;
//   stackless — a step function per process, one scheduler, a run queue.
// node ring.mjs MODE N T M
const CAP = 16;

class Mailbox {
  constructor() { this.q = []; this.waiter = null; }
  send(v) {
    if (this.waiter) { const w = this.waiter; this.waiter = null; w(v); }
    else this.q.push(v);
  }
  recv() {
    if (this.q.length) return Promise.resolve(this.q.shift());
    return new Promise(r => { this.waiter = r; });
  }
}

async function native(n, t, m) {
  const mb = Array.from({ length: n }, () => new Mailbox());
  for (let i = 1; i < n; i++) {
    const inb = mb[i], out = mb[(i + 1) % n];
    (async () => { for (;;) out.send(await inb.recv()); })();
  }
  const start = process.hrtime.bigint();
  await new Promise(done => {
    (async () => {
      const inb = mb[0], out = mb[1 % n];
      for (let k = 0; k < t; k++) out.send(m);
      let live = t;
      for (;;) {
        const v = await inb.recv();
        if (v === 1) { if (--live === 0) { done(); return; } continue; }
        out.send(v - 1);
      }
    })();
  });
  return Number(process.hrtime.bigint() - start);
}

function stackless(n, t, m) {
  const mbox = new Int32Array(n * CAP), head = new Int32Array(n), size = new Int32Array(n);
  const runq = new Int32Array(n);
  let rh = 0, rs = 0;
  const push = (a, v) => {
    mbox[a * CAP + ((head[a] + size[a]) % CAP)] = v;
    if (++size[a] === 1) { runq[(rh + rs) % n] = a; rs++; }
  };
  const start = process.hrtime.bigint();
  for (let k = 0; k < t; k++) push(1 % n, m);
  let live = t;
  while (rs > 0) {
    const a = runq[rh]; rh = (rh + 1) % n; rs--;
    while (size[a] > 0) {
      let v = mbox[a * CAP + head[a]];
      head[a] = (head[a] + 1) % CAP; size[a]--;
      if (a === 0) { if (v === 1) { live--; continue; } v--; }
      push((a + 1) % n, v);
    }
  }
  if (live !== 0) throw new Error("lost a token");
  return Number(process.hrtime.bigint() - start);
}

const [mode, ns, ts, ms] = process.argv.slice(2);
const n = +ns, t = +ts, m = +ms;
const run = mode === "stackless" ? stackless : native;
await run(n, t, Math.floor(m / 10));
const ds = [];
for (let r = 0; r < 7; r++) ds.push((await run(n, t, m)) / (n * t * m));
ds.sort((a, b) => a - b);
console.log(`js ${mode} N=${n} T=${t} M=${m}: ${ds[3].toFixed(1)} ns/hop (min ${ds[0].toFixed(1)} max ${ds[6].toFixed(1)})`);
