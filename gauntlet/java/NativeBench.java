/**
 * The gauntlet on the NATIVE Java target, against hand-written Java.
 *
 * Java is the host that had not moved. Every Java number in this repository
 * until now came from the retired portable layer, and Java is the one most
 * likely to disagree with Go and JavaScript: it is where the fused `merge`
 * LOSES 2.6x exactly where Go's fused `m[k]++` wins 1.19x.
 *
 * The generated code comes from examples/native/*-java.oro, which are written
 * on the language's own table — `(table n f)`, `len`, and indexing by
 * application — not on a hand-rolled vector library.
 *
 * One thing this exists to measure: our `int` maps to Java's `long`, so an
 * emitted loop counter is a `long` and every array access carries an `(int)`
 * cast. A person writes `int`. Whether that costs is the question Java has
 * never been asked.
 */
public final class NativeBench {

    static final int NVEC = 1 << 16;
    static final int NSMALL = 1024;
    static double sink;
    static long lsink;

    // ---- hand-written references, in the shape a person writes -------------

    static double dotRef(double[] xs, double[] ys) {
        double acc = 0.0;
        for (int i = 0; i < xs.length; i++) acc += xs[i] * ys[i];
        return acc;
    }

    /** The same, with a `long` counter — isolating the cast, not the shape. */
    static double dotLongRef(double[] xs, double[] ys) {
        double acc = 0.0;
        for (long i = 0; i < xs.length; i++) acc += xs[(int) i] * ys[(int) i];
        return acc;
    }

    static long findFirstRef(double[] a, double k) {
        for (int i = 0; i < a.length; i++) if (a[i] > k) return i;
        return -1;
    }

    /** long counter, direct return -- isolates the cast from the exit shape. */
    static long findFirstLongRef(double[] a, double k) {
        for (long i = 0; i < a.length; i++) if (a[(int) i] > k) return i;
        return -1;
    }

    /**
     * long counter AND a result variable with `break`, which is what we emit.
     * native-js-2026-08-20 found this shape costs 1.31x on V8 against an early
     * `return`, and the fix was applied to JavaScript only.
     */
    static long findFirstResultRef(double[] a, double k) {
        long i = 0;
        long r = 0;
        for (;; i = i + 1) {
            if (i >= a.length) { r = -1; break; }
            if (a[(int) i] > k) { r = i; break; }
            continue;
        }
        return r;
    }

    static double centroidRef(double[] xs, double[] ys) {
        double ax = 0.0, ay = 0.0;
        for (int i = 0; i < xs.length; i++) { ax += xs[i]; ay += ys[i]; }
        return ax + ay;
    }

    static double centroidLongRef(double[] xs, double[] ys) {
        double ax = 0.0, ay = 0.0;
        for (long i = 0; i < xs.length; i++) { ax += xs[(int) i]; ay += ys[(int) i]; }
        return ax + ay;
    }

    /** THE PARASITE TEST. Java's UNFUSED idiom, which measured 2.6x faster
     *  than the fused `merge` — the opposite of Go. */
    static java.util.Map<String,Long> wcRef(String text) {
        String[] ws = text.split(" ");
        java.util.Map<String,Long> m = new java.util.HashMap<String,Long>();
        for (int i = 0; i < ws.length; i++) {
            String w = ws[i];
            m.put(w, m.getOrDefault(w, 0L) + 1);
        }
        return m;
    }

    /** The FUSED form, carried because a losing form that is not measured is a
     *  belief rather than a result. */
    static java.util.Map<String,Long> wcMergeRef(String text) {
        String[] ws = text.split(" ");
        java.util.Map<String,Long> m = new java.util.HashMap<String,Long>();
        for (int i = 0; i < ws.length; i++) m.merge(ws[i], 1L, Long::sum);
        return m;
    }

    /** unfused, with a long counter and the cast -- our shape. */
    static java.util.Map<String,Long> wcLongRef(String text) {
        String[] ws = text.split(" ");
        java.util.Map<String,Long> m = new java.util.HashMap<String,Long>();
        for (long i = 0; i < ws.length; i++) {
            String w = ws[(int) i];
            m.put(w, m.getOrDefault(w, 0L) + 1);
        }
        return m;
    }

    /** The ORIGINAL shape baseline R5 measured: Integer, not Long. */
    static java.util.Map<String,Integer> wcIntUnfusedRef(String text) {
        String[] ws = text.split(" ");
        java.util.Map<String,Integer> m = new java.util.HashMap<String,Integer>();
        for (int i = 0; i < ws.length; i++) m.put(ws[i], m.getOrDefault(ws[i], 0) + 1);
        return m;
    }

    static java.util.Map<String,Integer> wcIntMergeRef(String text) {
        String[] ws = text.split(" ");
        java.util.Map<String,Integer> m = new java.util.HashMap<String,Integer>();
        for (int i = 0; i < ws.length; i++) m.merge(ws[i], 1, Integer::sum);
        return m;
    }

    // ---- harness -----------------------------------------------------------

    /**
     * Warm-up is bounded by TIME, not by a fixed count. A fixed 50,000
     * iterations is fine for a 450 ns `dot` and takes three minutes for a
     * 3.4 ms wordcount, which is how the first version of this file ran for
     * seventeen minutes. One second is past C2's compilation thresholds for
     * everything measured here.
     */
    static void bench(String name, Runnable fn, int iters) {
        long warm = System.nanoTime();
        int w = 0;
        while (System.nanoTime() - warm < 1_000_000_000L && w < 200_000) { fn.run(); w++; }
        double[] ts = new double[9];
        for (int r = 0; r < 9; r++) {
            long t0 = System.nanoTime();
            for (int i = 0; i < iters; i++) fn.run();
            ts[r] = (System.nanoTime() - t0) / (double) iters;
        }
        java.util.Arrays.sort(ts);
        System.out.printf("%-34s %10.1f ns   (min %.1f)%n", name, ts[4], ts[0]);
    }

    static void eq(double a, double b, String what) {
        if (Math.abs(a - b) > 1e-9 * Math.max(1, Math.abs(b)))
            throw new AssertionError(what + ": " + a + " != " + b);
    }

    public static void main(String[] args) {
        double[] A = Gauntlet.makeVec(NVEC, 1);
        double[] B = Gauntlet.makeVec(NVEC, 2);
        double[] sA = Gauntlet.makeVec(NSMALL, 1);
        double[] sB = Gauntlet.makeVec(NSMALL, 2);

        // Correctness first. A benchmark of a wrong program is not a result.
        eq(NatDot.NatDotDot(A, B), dotRef(A, B), "dot");
        eq(NatCentroid.NatCentroidCentroidX(A, B), centroidRef(A, B), "centroid");
        if (NatSearch.NatSearchFindFirst(A, 0.5) != findFirstRef(A, 0.5))
            throw new AssertionError("search");
        if (NatSearch.NatSearchFindFirst(A, 2.0) != findFirstRef(A, 2.0))
            throw new AssertionError("search late");
        System.out.println("agreement: ok\n");

        bench("dot  n=65536  hand-written", () -> sink = dotRef(A, B), 500);
        bench("dot  n=65536  hand, long counter", () -> sink = dotLongRef(A, B), 500);
        bench("dot  n=65536  GENERATED", () -> sink = NatDot.NatDotDot(A, B), 500);
        System.out.println();
        bench("dot  n=1024   hand-written", () -> sink = dotRef(sA, sB), 50000);
        bench("dot  n=1024   hand, long counter", () -> sink = dotLongRef(sA, sB), 50000);
        bench("dot  n=1024   GENERATED", () -> sink = NatDot.NatDotDot(sA, sB), 50000);
        System.out.println();
        bench("centroid      hand-written", () -> sink = centroidRef(A, B), 500);
        bench("centroid      hand, long counter", () -> sink = centroidLongRef(A, B), 500);
        bench("centroid      GENERATED",
                () -> sink = NatCentroid.NatCentroidCentroidX(A, B), 500);
        System.out.println();
        String text = Gauntlet.makeText(NVEC, 5);
        if (!NatWc.NatWcTally(text).equals(wcRef(text))) throw new AssertionError("wordcount");
        if (!wcMergeRef(text).equals(wcRef(text))) throw new AssertionError("wordcount merge");
        bench("wordcount     hand, unfused", () -> sink = wcRef(text).size(), 200);
        bench("wordcount     hand, FUSED merge", () -> sink = wcMergeRef(text).size(), 200);
        bench("wordcount     hand, long counter", () -> sink = wcLongRef(text).size(), 200);
        bench("wordcount     hand, Integer unfused", () -> sink = wcIntUnfusedRef(text).size(), 200);
        bench("wordcount     hand, Integer MERGE", () -> sink = wcIntMergeRef(text).size(), 200);
        bench("wordcount     GENERATED", () -> sink = NatWc.NatWcTally(text).size(), 200);
        System.out.println();
        bench("search early  hand-written", () -> lsink = findFirstRef(A, 0.5), 200000);
        bench("search early  hand, long counter", () -> lsink = findFirstLongRef(A, 0.5), 200000);
        bench("search early  GENERATED",
                () -> lsink = NatSearch.NatSearchFindFirst(A, 0.5), 200000);
        bench("search late   hand-written", () -> lsink = findFirstRef(A, 2.0), 2000);
        bench("search late   hand, long counter", () -> lsink = findFirstLongRef(A, 2.0), 2000);
        bench("search late   hand, result+break", () -> lsink = findFirstResultRef(A, 2.0), 2000);
        bench("search late   GENERATED",
                () -> lsink = NatSearch.NatSearchFindFirst(A, 2.0), 2000);
    }
}
