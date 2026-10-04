// Memory per idle process: K async functions each awaiting its own mailbox.
const K = 100000;
class Mailbox { constructor() { this.q = []; this.waiter = null; }
  recv() { return new Promise(r => { this.waiter = r; }); } }
global.gc(); const a = process.memoryUsage().heapUsed;
const mb = [];
for (let i = 0; i < K; i++) { const m = new Mailbox(); mb.push(m); (async () => { await m.recv(); })(); }
await new Promise(r => setTimeout(r, 50));
global.gc(); const b = process.memoryUsage().heapUsed;
console.log(`js native: ${Math.round((b - a) / K)} bytes per idle process (heap ${a} -> ${b}, ${mb.length} mailboxes, waiting ${mb.filter(m => m.waiter).length})`);
