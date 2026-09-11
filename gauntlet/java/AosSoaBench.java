/**
 * AoS against SoA on the node table, on the JVM. See gauntlet/go/aossoa.go for
 * the question and the design; the same three layouts and the same two
 * workloads.
 *
 *   flat   int[] nodes, 4*n+f          what the product pass emits (at its width)
 *   SoA    four int[]
 *   AoS    Node[] of objects with int fields -- the only record a JDK 17 array
 *          can hold, since there are no value types before Valhalla
 *
 * ELEMENT WIDTH IS HELD AT int ACROSS ALL THREE. json-tree-bench found this host
 * is the one sensitive to width, and rebench-2026-08-27 priced the node table's
 * width at ~20% here; a comparison that changed width and layout together would
 * measure neither, which this repository has now done three times.
 *
 * g2 measured Point[] at only 1.05x against parallel arrays in August, because
 * HotSpot's bump allocator lays freshly built objects out contiguously. That is
 * the claim this re-takes, on a program where the objects are built in a loop
 * and then walked in an order the data decides.
 *
 *   javac -d out AosSoaBench.java && java -cp out AosSoaBench
 */
public final class AosSoaBench {

    static final int NMAX = 512;
    static final int DMAX = 32;
    static final int SCAN_N = 65536;
    static long sink;

    static boolean isNum(long c) {
        return (c >= 48 && c <= 57) || c == 45 || c == 43 || c == 46 || c == 101 || c == 69;
    }
    static boolean isAlpha(long c) { return c >= 97 && c <= 122; }
    static boolean isSkip(long c) {
        return c == 32 || c == 9 || c == 10 || c == 13 || c == 58 || c == 44;
    }
    static int scanString(long[] a, int i) {
        int j = i + 1;
        for (;;) {
            if (j >= a.length) return j;
            if (a[j] == 92) { j += 2; continue; }
            if (a[j] == 34) return j + 1;
            j++;
        }
    }

    // ---- W1, flat -----------------------------------------------------------

    static long treeFlat(long[] a) {
        int[] t = new int[4 * NMAX];
        int[] stk = new int[2 * DMAX];
        int i = 0, nn = 1, sp = 0;
        for (;;) {
            if (i >= a.length || nn >= NMAX || sp >= DMAX) break;
            long c = a[i];
            if (isSkip(c)) { i++; continue; }
            if (c == 123 || c == 91) {
                t[4 * nn] = c == 123 ? 5 : 4; t[4 * nn + 1] = 0;
                if (sp >= 1) { int lc = stk[2 * (sp - 1) + 1];
                    if (lc == 0) t[4 * stk[2 * (sp - 1)] + 2] = nn; else t[4 * lc + 3] = nn; }
                if (sp >= 1) stk[2 * (sp - 1) + 1] = nn;
                stk[2 * sp] = nn; stk[2 * sp + 1] = 0;
                i++; sp++; nn++;
                continue;
            }
            if (c == 125 || c == 93) { i++; if (sp >= 1) sp--; continue; }
            if (c == 34 || isNum(c) || isAlpha(c)) {
                int tg = 2, ni;
                if (c == 34) ni = scanString(a, i);
                else if (isNum(c)) { tg = 1; int j = i; while (j < a.length && isNum(a[j])) j++; ni = j; }
                else { tg = 3; int j = i; while (j < a.length && isAlpha(a[j])) j++; ni = j; }
                t[4 * nn] = tg; t[4 * nn + 1] = ni - i;
                if (sp >= 1) { int lc = stk[2 * (sp - 1) + 1];
                    if (lc == 0) t[4 * stk[2 * (sp - 1)] + 2] = nn; else t[4 * lc + 3] = nn; }
                if (sp >= 1) stk[2 * (sp - 1) + 1] = nn;
                i = ni; nn++;
                continue;
            }
            i++;
        }
        int[] wl = new int[2 * NMAX];
        wl[0] = 1; wl[1] = 1;
        int wp = 1, seen = 0, steps = 0;
        long acc = 0;
        while (wp >= 1 && steps < 2 * NMAX) {
            int n = wl[2 * (wp - 1)], d = wl[2 * (wp - 1) + 1];
            int sb = t[4 * n + 3], kd = t[4 * n + 2];
            wp--;
            if (sb != 0) { wl[2 * wp] = sb; wl[2 * wp + 1] = d; wp++; }
            if (kd != 0) { wl[2 * wp] = kd; wl[2 * wp + 1] = d + 1; wp++; }
            seen++; acc += (long) t[4 * n] * d; steps++;
        }
        return seen * 1000L + acc;
    }

    // ---- W1, SoA ------------------------------------------------------------

    static long treeSoA(long[] a) {
        int[] tag = new int[NMAX], val = new int[NMAX], kid = new int[NMAX], sib = new int[NMAX];
        int[] stk = new int[2 * DMAX];
        int i = 0, nn = 1, sp = 0;
        for (;;) {
            if (i >= a.length || nn >= NMAX || sp >= DMAX) break;
            long c = a[i];
            if (isSkip(c)) { i++; continue; }
            if (c == 123 || c == 91) {
                tag[nn] = c == 123 ? 5 : 4; val[nn] = 0;
                if (sp >= 1) { int lc = stk[2 * (sp - 1) + 1];
                    if (lc == 0) kid[stk[2 * (sp - 1)]] = nn; else sib[lc] = nn; }
                if (sp >= 1) stk[2 * (sp - 1) + 1] = nn;
                stk[2 * sp] = nn; stk[2 * sp + 1] = 0;
                i++; sp++; nn++;
                continue;
            }
            if (c == 125 || c == 93) { i++; if (sp >= 1) sp--; continue; }
            if (c == 34 || isNum(c) || isAlpha(c)) {
                int tg = 2, ni;
                if (c == 34) ni = scanString(a, i);
                else if (isNum(c)) { tg = 1; int j = i; while (j < a.length && isNum(a[j])) j++; ni = j; }
                else { tg = 3; int j = i; while (j < a.length && isAlpha(a[j])) j++; ni = j; }
                tag[nn] = tg; val[nn] = ni - i;
                if (sp >= 1) { int lc = stk[2 * (sp - 1) + 1];
                    if (lc == 0) kid[stk[2 * (sp - 1)]] = nn; else sib[lc] = nn; }
                if (sp >= 1) stk[2 * (sp - 1) + 1] = nn;
                i = ni; nn++;
                continue;
            }
            i++;
        }
        int[] wl = new int[2 * NMAX];
        wl[0] = 1; wl[1] = 1;
        int wp = 1, seen = 0, steps = 0;
        long acc = 0;
        while (wp >= 1 && steps < 2 * NMAX) {
            int n = wl[2 * (wp - 1)], d = wl[2 * (wp - 1) + 1];
            int sb = sib[n], kd = kid[n];
            wp--;
            if (sb != 0) { wl[2 * wp] = sb; wl[2 * wp + 1] = d; wp++; }
            if (kd != 0) { wl[2 * wp] = kd; wl[2 * wp + 1] = d + 1; wp++; }
            seen++; acc += (long) tag[n] * d; steps++;
        }
        return seen * 1000L + acc;
    }

    // ---- W1, AoS objects ----------------------------------------------------

    static final class Node { int tag, val, kid, sib; }

    static long treeAoS(long[] a) {
        // Every slot allocated up front and in order, which is what lets the
        // TLAB lay them out contiguously -- the best case for this layout, and
        // the fair one to measure.
        Node[] ns = new Node[NMAX];
        for (int k = 0; k < NMAX; k++) ns[k] = new Node();
        int[] stk = new int[2 * DMAX];
        int i = 0, nn = 1, sp = 0;
        for (;;) {
            if (i >= a.length || nn >= NMAX || sp >= DMAX) break;
            long c = a[i];
            if (isSkip(c)) { i++; continue; }
            if (c == 123 || c == 91) {
                ns[nn].tag = c == 123 ? 5 : 4; ns[nn].val = 0;
                if (sp >= 1) { int lc = stk[2 * (sp - 1) + 1];
                    if (lc == 0) ns[stk[2 * (sp - 1)]].kid = nn; else ns[lc].sib = nn; }
                if (sp >= 1) stk[2 * (sp - 1) + 1] = nn;
                stk[2 * sp] = nn; stk[2 * sp + 1] = 0;
                i++; sp++; nn++;
                continue;
            }
            if (c == 125 || c == 93) { i++; if (sp >= 1) sp--; continue; }
            if (c == 34 || isNum(c) || isAlpha(c)) {
                int tg = 2, ni;
                if (c == 34) ni = scanString(a, i);
                else if (isNum(c)) { tg = 1; int j = i; while (j < a.length && isNum(a[j])) j++; ni = j; }
                else { tg = 3; int j = i; while (j < a.length && isAlpha(a[j])) j++; ni = j; }
                ns[nn].tag = tg; ns[nn].val = ni - i;
                if (sp >= 1) { int lc = stk[2 * (sp - 1) + 1];
                    if (lc == 0) ns[stk[2 * (sp - 1)]].kid = nn; else ns[lc].sib = nn; }
                if (sp >= 1) stk[2 * (sp - 1) + 1] = nn;
                i = ni; nn++;
                continue;
            }
            i++;
        }
        int[] wl = new int[2 * NMAX];
        wl[0] = 1; wl[1] = 1;
        int wp = 1, seen = 0, steps = 0;
        long acc = 0;
        while (wp >= 1 && steps < 2 * NMAX) {
            int n = wl[2 * (wp - 1)], d = wl[2 * (wp - 1) + 1];
            Node x = ns[n];
            int sb = x.sib, kd = x.kid;
            wp--;
            if (sb != 0) { wl[2 * wp] = sb; wl[2 * wp + 1] = d; wp++; }
            if (kd != 0) { wl[2 * wp] = kd; wl[2 * wp + 1] = d + 1; wp++; }
            seen++; acc += (long) x.tag * d; steps++;
        }
        return seen * 1000L + acc;
    }

    // ---- W2 -----------------------------------------------------------------

    static int fTag(int i) { return (i * 7) % 5 + 1; }
    static int fVal(int i) { return i % 97; }

    static long scanFlat(int[] t) {
        long s = 0;
        for (int i = 0; i + 3 < t.length; i += 4) if (t[i] == 2) s += t[i + 1];
        return s;
    }
    static long scanSoA(int[] tag, int[] val) {
        long s = 0;
        for (int i = 0; i < tag.length; i++) if (tag[i] == 2) s += val[i];
        return s;
    }
    static long scanAoS(Node[] ns) {
        long s = 0;
        for (int i = 0; i < ns.length; i++) { Node x = ns[i]; if (x.tag == 2) s += x.val; }
        return s;
    }

    // ---- W3 -----------------------------------------------------------------
    //
    // THE CASE AoS EXISTS FOR -- random nodes of a table far past cache, all
    // four fields read. W1 has that access pattern at 443 nodes, which is
    // L1-resident and so cannot test locality; see gauntlet/go/aossoa.go. On
    // this host AoS is an array of REFERENCES, so a random node costs the
    // reference load and then the object: g2's 1.05x was measured on objects
    // walked in allocation order, which is the case a TLAB lays out
    // contiguously, and this is the case it does not help.

    static final int GATHER_N = 1 << 20;
    static final int GATHER_M = 1 << 16;

    static int[] gatherIdx() {
        int[] idx = new int[GATHER_M];
        long x = 1;
        for (int i = 0; i < GATHER_M; i++) { x = x * 48271 % 2147483647L; idx[i] = (int) (x & (GATHER_N - 1)); }
        return idx;
    }

    static long gatherFlat(int[] t, int[] idx) {
        long s = 0;
        for (int k : idx) s += (long) t[4 * k] + t[4 * k + 1] + t[4 * k + 2] + t[4 * k + 3];
        return s;
    }
    static long gatherSoA(int[] tag, int[] val, int[] kid, int[] sib, int[] idx) {
        long s = 0;
        for (int k : idx) s += (long) tag[k] + val[k] + kid[k] + sib[k];
        return s;
    }
    static long gatherAoS(Node[] ns, int[] idx) {
        long s = 0;
        for (int k : idx) { Node n = ns[k]; s += (long) n.tag + n.val + n.kid + n.sib; }
        return s;
    }

    // ---- harness ------------------------------------------------------------

    static String makeDoc(int records) {
        StringBuilder b = new StringBuilder("{\"items\":[");
        for (int r = 0; r < records; r++) {
            if (r > 0) b.append(',');
            b.append("{\"id\":1234,\"name\":\"a b\\\"c\",\"tags\":[\"x\",\"y\",\"z\"],")
             .append("\"score\":-12.5e3,\"ok\":true,\"prev\":null,")
             .append("\"meta\":{\"depth\":2,\"flag\":false}}");
        }
        return b.append("]}").toString();
    }
    static long[] longsOf(String s) {
        long[] out = new long[s.length()];
        for (int i = 0; i < s.length(); i++) out[i] = s.charAt(i);
        return out;
    }

    static void run(String what, java.util.function.LongSupplier f, int warm, int iters) {
        for (int i = 0; i < warm; i++) sink ^= f.getAsLong();
        double[] ts = new double[9];
        for (int r = 0; r < 9; r++) {
            long t0 = System.nanoTime();
            for (int i = 0; i < iters; i++) sink ^= f.getAsLong();
            ts[r] = (System.nanoTime() - t0) / (double) iters;
        }
        java.util.Arrays.sort(ts);
        // MEDIAN, not best: best-of-9 flatters whichever form has the fattest
        // tail, and the question here is a ratio between forms.
        System.out.printf("%-28s %12.1f ns/op%n", what, ts[4]);
    }

    public static void main(String[] args) {
        for (int n : new int[]{0, 1, 2, 5, 20}) {
            long[] d = longsOf(makeDoc(n));
            long f = treeFlat(d), s = treeSoA(d), o = treeAoS(d);
            if (f != s || f != o) throw new AssertionError("records=" + n + ": " + f + " " + s + " " + o);
        }
        int[] flat = new int[4 * SCAN_N], tag = new int[SCAN_N], val = new int[SCAN_N];
        Node[] aos = new Node[SCAN_N];
        long want = 0;
        for (int i = 0; i < SCAN_N; i++) {
            int tg = fTag(i), vl = fVal(i), kd = (i + 1) % SCAN_N, sb = (i * 3) % SCAN_N;
            flat[4 * i] = tg; flat[4 * i + 1] = vl; flat[4 * i + 2] = kd; flat[4 * i + 3] = sb;
            tag[i] = tg; val[i] = vl;
            Node x = new Node(); x.tag = tg; x.val = vl; x.kid = kd; x.sib = sb; aos[i] = x;
            if (tg == 2) want += vl;
        }
        if (scanFlat(flat) != want || scanSoA(tag, val) != want || scanAoS(aos) != want || want == 0)
            throw new AssertionError("scan disagrees");

        int[] gflat = new int[4 * GATHER_N], gtag = new int[GATHER_N], gval = new int[GATHER_N];
        int[] gkid = new int[GATHER_N], gsib = new int[GATHER_N];
        Node[] gaos = new Node[GATHER_N];
        for (int i = 0; i < GATHER_N; i++) {
            int tg = fTag(i), vl = fVal(i), kd = (i + 1) % GATHER_N, sb = (i * 3) % GATHER_N;
            gflat[4 * i] = tg; gflat[4 * i + 1] = vl; gflat[4 * i + 2] = kd; gflat[4 * i + 3] = sb;
            gtag[i] = tg; gval[i] = vl; gkid[i] = kd; gsib[i] = sb;
            Node x = new Node(); x.tag = tg; x.val = vl; x.kid = kd; x.sib = sb; gaos[i] = x;
        }
        int[] gi = gatherIdx();
        long gwant = 0;
        for (int k : gi) gwant += (long) fTag(k) + fVal(k) + (k + 1) % GATHER_N + (long) (k * 3) % GATHER_N;
        if (gatherFlat(gflat, gi) != gwant || gatherSoA(gtag, gval, gkid, gsib, gi) != gwant
                || gatherAoS(gaos, gi) != gwant || gwant == 0)
            throw new AssertionError("gather disagrees");
        System.out.println("all layouts agree");
        if (args.length > 0 && args[0].equals("--check")) return;

        long[] doc = longsOf(makeDoc(20));
        String which = args.length > 0 ? args[0] : "all";
        if (which.equals("all") || which.equals("tree-flat")) run("W1 tree  flat", () -> treeFlat(doc), 50000, 2000);
        if (which.equals("all") || which.equals("tree-soa"))  run("W1 tree  SoA", () -> treeSoA(doc), 50000, 2000);
        if (which.equals("all") || which.equals("tree-aos"))  run("W1 tree  AoS objects", () -> treeAoS(doc), 50000, 2000);
        if (which.equals("all") || which.equals("scan-flat")) run("W2 scan  flat", () -> scanFlat(flat), 2000, 200);
        if (which.equals("all") || which.equals("scan-soa"))  run("W2 scan  SoA", () -> scanSoA(tag, val), 2000, 200);
        if (which.equals("all") || which.equals("scan-aos"))  run("W2 scan  AoS objects", () -> scanAoS(aos), 2000, 200);
        if (which.equals("all") || which.equals("gather-flat")) run("W3 gather flat", () -> gatherFlat(gflat, gi), 50, 20);
        if (which.equals("all") || which.equals("gather-soa"))  run("W3 gather SoA", () -> gatherSoA(gtag, gval, gkid, gsib, gi), 50, 20);
        if (which.equals("all") || which.equals("gather-aos"))  run("W3 gather AoS objects", () -> gatherAoS(gaos, gi), 50, 20);
    }
}
