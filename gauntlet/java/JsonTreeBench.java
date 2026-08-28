/**
 * A JSON tree on Java: emitted against hand-written.
 *
 * THREE FORMS, and the first two are the question:
 *
 *   rec     recursive descent into a linked node object -- what a person writes
 *   flat    a flat node table plus indices with an explicit stack -- our shape
 *   GEN     emitted from the .oro
 *
 * `rec` against `flat` prices the REPRESENTATION, and this is the host where
 * the answer is least predictable. The JVM allocates by bumping a pointer in a
 * thread-local buffer, its young collector is a copying collector that pays for
 * survivors rather than for garbage, and C2 can scalar-replace an object that
 * does not escape. All three work against the flat table. Go measured the flat
 * form 2.52x faster and JavaScript only 1.22x; ADR 0008 says take the
 * measurement rather than the principle.
 *
 *   javac -encoding UTF-8 -d out gen/GenJsonTree.java JsonTreeBench.java
 *   java -cp out JsonTreeBench
 */
public final class JsonTreeBench {

    static final int NMAX = 512;
    static final int DMAX = 32;
    static Object sink;

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

    // ---- recursive, boxed ---------------------------------------------------

    static final class Node {
        int tag, val;
        Node kid, sib;
        Node(int tag, int val) { this.tag = tag; this.val = val; }
    }

    static int pos;

    static int skip(long[] a, int i) {
        while (i < a.length && isSkip(a[i])) i++;
        return i;
    }

    static Node parseValue(long[] a, int i) {
        if (i >= a.length) { pos = i; return null; }
        long c = a[i];
        if (c == 123 || c == 91) {
            Node n = new Node(c == 123 ? 5 : 4, 0);
            i++;
            Node last = null;
            for (;;) {
                i = skip(a, i);
                if (i >= a.length || a[i] == 125 || a[i] == 93) {
                    if (i < a.length) i++;
                    break;
                }
                Node child = parseValue(a, i);
                if (child == null) break;
                i = pos;
                if (last == null) n.kid = child; else last.sib = child;
                last = child;
            }
            pos = i;
            return n;
        }
        if (c == 34) { int ni = scanString(a, i); pos = ni; return new Node(2, ni - i); }
        if (isNum(c)) {
            int j = i;
            while (j < a.length && isNum(a[j])) j++;
            pos = j;
            return new Node(1, j - i);
        }
        if (isAlpha(c)) {
            int j = i;
            while (j < a.length && isAlpha(a[j])) j++;
            pos = j;
            return new Node(3, j - i);
        }
        pos = i + 1;
        return null;
    }

    static int seenRec, accRec;

    static void walkRec(Node n, int d) {
        if (n == null) return;
        seenRec++;
        accRec += n.tag * d;
        for (Node c = n.kid; c != null; c = c.sib) walkRec(c, d + 1);
    }

    static long treeRec(long[] a) {
        Node root = parseValue(a, skip(a, 0));
        seenRec = 0; accRec = 0;
        walkRec(root, 1);
        return seenRec * 1000L + accRec;
    }

    // ---- flat, indexed ------------------------------------------------------

    static long treeFlat(long[] a) {
        long[] nodes = new long[4 * NMAX];
        long[] stk = new long[2 * DMAX];
        int i = 0, nn = 1, sp = 0;
        for (;;) {
            if (i >= a.length || nn >= NMAX || sp >= DMAX) break;
            long c = a[i];
            if (isSkip(c)) { i++; continue; }
            if (c == 123 || c == 91) {
                nodes[4 * nn] = c == 123 ? 5 : 4;
                nodes[4 * nn + 1] = 0;
                link(nodes, stk, sp, nn);
                if (sp >= 1) stk[2 * (sp - 1) + 1] = nn;
                stk[2 * sp] = nn;
                stk[2 * sp + 1] = 0;
                i++; sp++; nn++;
                continue;
            }
            if (c == 125 || c == 93) { i++; if (sp >= 1) sp--; continue; }
            if (c == 34 || isNum(c) || isAlpha(c)) {
                int tg = 2, ni;
                if (c == 34) { ni = scanString(a, i); }
                else if (isNum(c)) {
                    tg = 1;
                    int j = i;
                    while (j < a.length && isNum(a[j])) j++;
                    ni = j;
                } else {
                    tg = 3;
                    int j = i;
                    while (j < a.length && isAlpha(a[j])) j++;
                    ni = j;
                }
                nodes[4 * nn] = tg;
                nodes[4 * nn + 1] = ni - i;
                link(nodes, stk, sp, nn);
                if (sp >= 1) stk[2 * (sp - 1) + 1] = nn;
                i = ni; nn++;
                continue;
            }
            i++;
        }
        return walkFlat(nodes);
    }

    static void link(long[] nodes, long[] stk, int sp, int k) {
        if (sp < 1) return;
        long lc = stk[2 * (sp - 1) + 1];
        if (lc == 0) nodes[(int) (4 * stk[2 * (sp - 1)]) + 2] = k;
        else nodes[(int) (4 * lc) + 3] = k;
    }

    static long walkFlat(long[] nodes) {
        long[] wl = new long[2 * NMAX];
        wl[0] = 1; wl[1] = 1;
        int sp = 1, seen = 0, steps = 0;
        long acc = 0;
        while (sp >= 1 && steps < 2 * NMAX) {
            int n = (int) wl[2 * (sp - 1)];
            long d = wl[2 * (sp - 1) + 1];
            long sb = nodes[4 * n + 3], kd = nodes[4 * n + 2];
            sp--;
            if (sb != 0) { wl[2 * sp] = sb; wl[2 * sp + 1] = d; sp++; }
            if (kd != 0) { wl[2 * sp] = kd; wl[2 * sp + 1] = d + 1; sp++; }
            seen++;
            acc += nodes[4 * n] * d;
            steps++;
        }
        return seen * 1000L + acc;
    }

    // THE SAME FLAT TABLE IN int[], which is what a person would actually
    // write here. Our `int` is 64-bit (ADR 0012) so we emit long[], and on this
    // host that is 32 bytes per node against a Node object's 24 with compressed
    // oops -- the flat form is LARGER than the boxed one, which is not true on
    // Go. Carrying both separates the element width from the representation.

    static long treeFlatInt(long[] a) {
        int[] nodes = new int[4 * NMAX];
        int[] stk = new int[2 * DMAX];
        int i = 0, nn = 1, sp = 0;
        for (;;) {
            if (i >= a.length || nn >= NMAX || sp >= DMAX) break;
            long c = a[i];
            if (isSkip(c)) { i++; continue; }
            if (c == 123 || c == 91) {
                nodes[4 * nn] = c == 123 ? 5 : 4;
                nodes[4 * nn + 1] = 0;
                linkInt(nodes, stk, sp, nn);
                if (sp >= 1) stk[2 * (sp - 1) + 1] = nn;
                stk[2 * sp] = nn;
                stk[2 * sp + 1] = 0;
                i++; sp++; nn++;
                continue;
            }
            if (c == 125 || c == 93) { i++; if (sp >= 1) sp--; continue; }
            if (c == 34 || isNum(c) || isAlpha(c)) {
                int tg = 2, ni;
                if (c == 34) { ni = scanString(a, i); }
                else if (isNum(c)) {
                    tg = 1;
                    int j = i;
                    while (j < a.length && isNum(a[j])) j++;
                    ni = j;
                } else {
                    tg = 3;
                    int j = i;
                    while (j < a.length && isAlpha(a[j])) j++;
                    ni = j;
                }
                nodes[4 * nn] = tg;
                nodes[4 * nn + 1] = ni - i;
                linkInt(nodes, stk, sp, nn);
                if (sp >= 1) stk[2 * (sp - 1) + 1] = nn;
                i = ni; nn++;
                continue;
            }
            i++;
        }
        return walkFlatInt(nodes);
    }

    static void linkInt(int[] nodes, int[] stk, int sp, int k) {
        if (sp < 1) return;
        int lc = stk[2 * (sp - 1) + 1];
        if (lc == 0) nodes[4 * stk[2 * (sp - 1)] + 2] = k;
        else nodes[4 * lc + 3] = k;
    }

    static long walkFlatInt(int[] nodes) {
        int[] wl = new int[2 * NMAX];
        wl[0] = 1; wl[1] = 1;
        int sp = 1, seen = 0, steps = 0;
        long acc = 0;
        while (sp >= 1 && steps < 2 * NMAX) {
            int n = wl[2 * (sp - 1)];
            int d = wl[2 * (sp - 1) + 1];
            int sb = nodes[4 * n + 3], kd = nodes[4 * n + 2];
            sp--;
            if (sb != 0) { wl[2 * sp] = sb; wl[2 * sp + 1] = d; sp++; }
            if (kd != 0) { wl[2 * sp] = kd; wl[2 * sp + 1] = d + 1; sp++; }
            seen++;
            acc += nodes[4 * n] * d;
            steps++;
        }
        return seen * 1000L + acc;
    }

    // ---- input --------------------------------------------------------------

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

    // The generated parser takes short[] now: tree.oro declares (array (int 0
    // 255)) on its source, the JVM's byte is SIGNED so 0..255 does not fit it,
    // and short is the next representation up. The hand-written references keep
    // the long[] they were written against.
    static short[] shortsOf(String s) {
        short[] out = new short[s.length()];
        for (int i = 0; i < s.length(); i++) out[i] = (short) s.charAt(i);
        return out;
    }

    static long[] longsOf(String s) {
        long[] out = new long[s.length()];
        for (int i = 0; i < s.length(); i++) out[i] = s.charAt(i);
        return out;
    }

    static void run(String what, java.util.function.Supplier<Object> f, int warm, int iters) {
        for (int i = 0; i < warm; i++) sink = f.get();
        double best = Double.MAX_VALUE;
        for (int r = 0; r < 9; r++) {
            long t0 = System.nanoTime();
            for (int i = 0; i < iters; i++) sink = f.get();
            double d = (System.nanoTime() - t0) / (double) iters;
            if (d < best) best = d;
        }
        System.out.printf("%-32s %10.1f ns%n", what, best);
    }

    public static void main(String[] args) {
        for (int n : new int[]{0, 1, 2, 5, 20}) check(makeDoc(n));
        for (String s : new String[]{"[1,2]", "{\"a\":1}", "[[1],2]",
                "{\"a\":[1,2],\"b\":true}", "[]", "{}", "[[[[1]]]]"}) check(s);
        System.out.println("all three agree");

        long[] doc = longsOf(makeDoc(20));
        short[] ds = shortsOf(makeDoc(20));
        run("T  tree recursive     hand", () -> treeRec(doc), 50000, 2000);
        run("T  tree flat          hand", () -> treeFlat(doc), 50000, 2000);
        run("T  tree flat int[]    hand", () -> treeFlatInt(doc), 50000, 2000);
        run("T  tree flat          GEN ", () -> GenJsonTree.GenMeasure(ds), 50000, 2000);
    }

    static void check(String s) {
        long[] a = longsOf(s);
        long want = treeRec(a);
        if (treeFlat(a) != want || treeFlatInt(a) != want || GenJsonTree.GenMeasure(shortsOf(s)) != want) {
            throw new AssertionError(s + ": rec=" + want + " flat=" + treeFlat(a)
                    + " gen=" + GenJsonTree.GenMeasure(shortsOf(s)));
        }
    }
}
