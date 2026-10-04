// The ring on the JVM: N processes, T tokens, M rounds each.
//   native    — a platform thread per process, an ArrayBlockingQueue mailbox
//               (JDK 17 here: no virtual threads);
//   stackless — a step function per process, one scheduler, a run queue.
// javac Ring.java && java Ring MODE N T M
import java.util.*;
import java.util.concurrent.*;

public class Ring {
    static final int CAP = 16;

    static long nativeRun(int n, int t, int m) throws Exception {
        List<ArrayBlockingQueue<Integer>> q = new ArrayList<>();
        for (int i = 0; i < n; i++) q.add(new ArrayBlockingQueue<>(CAP));
        List<Thread> ts = new ArrayList<>();
        for (int i = 1; i < n; i++) {
            var in = q.get(i); var out = q.get((i + 1) % n);
            Thread th = new Thread(() -> {
                try { for (;;) { int v = in.take(); if (v < 0) return; out.put(v); } }
                catch (InterruptedException e) { }
            });
            th.setDaemon(true); th.start(); ts.add(th);
        }
        long start = System.nanoTime();
        var in = q.get(0); var out = q.get(1 % n);
        for (int k = 0; k < t; k++) out.put(m);
        int live = t;
        while (live > 0) {
            int v = in.take();
            if (v == 1) { live--; continue; }
            out.put(v - 1);
        }
        long el = System.nanoTime() - start;
        for (int i = 1; i < n; i++) q.get(i).put(-1);
        for (Thread th : ts) th.join();
        return el;
    }

    static long stackless(int n, int t, int m) {
        int[] mbox = new int[n * CAP], head = new int[n], size = new int[n], runq = new int[n];
        int rh = 0, rs = 0;
        long start = System.nanoTime();
        for (int k = 0; k < t; k++) {
            int a = 1 % n;
            mbox[a * CAP + (head[a] + size[a]) % CAP] = m;
            if (++size[a] == 1) { runq[(rh + rs) % n] = a; rs++; }
        }
        int live = t;
        while (rs > 0) {
            int a = runq[rh]; rh = (rh + 1) % n; rs--;
            while (size[a] > 0) {
                int v = mbox[a * CAP + head[a]];
                head[a] = (head[a] + 1) % CAP; size[a]--;
                if (a == 0) { if (v == 1) { live--; continue; } v--; }
                int b = (a + 1) % n;
                mbox[b * CAP + (head[b] + size[b]) % CAP] = v;
                if (++size[b] == 1) { runq[(rh + rs) % n] = b; rs++; }
            }
        }
        if (live != 0) throw new IllegalStateException("lost a token");
        return System.nanoTime() - start;
    }

    public static void main(String[] a) throws Exception {
        String mode = a[0];
        int n = Integer.parseInt(a[1]), t = Integer.parseInt(a[2]), m = Integer.parseInt(a[3]);
        boolean nat = !mode.equals("stackless");
        if (nat) nativeRun(n, t, m / 10); else stackless(n, t, m / 10);
        double[] ds = new double[7];
        for (int r = 0; r < 7; r++) {
            System.gc();
            ds[r] = (double) (nat ? nativeRun(n, t, m) : stackless(n, t, m)) / ((long) n * t * m);
        }
        Arrays.sort(ds);
        System.out.printf("java %s N=%d T=%d M=%d: %.1f ns/hop (min %.1f max %.1f)%n", mode, n, t, m, ds[3], ds[0], ds[6]);
    }
}
