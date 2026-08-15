import java.util.HashMap;

/** Generated Java against hand-written Java. */
public final class Parity {

    // ---- hand-written references ----
    static double dotRef(double[] xs, double[] ys) {
        double acc = 0.0;
        for (int i = 0; i < xs.length; i++) acc += xs[i] * ys[i];
        return acc;
    }
    static double filterRef(double[] a) {
        double acc = 0.0;
        for (int i = 0; i < a.length; i++) { double x = a[i]; if (x > 0) acc += x; }
        return acc;
    }
    static double centroidRef(double[] xs, double[] ys) {
        double ax = 0.0, ay = 0.0;
        for (int i = 0; i < xs.length; i++) { ax += xs[i]; ay += ys[i]; }
        return ax + ay;
    }
    static HashMap<String,Integer> wcRef(String text) {
        HashMap<String,Integer> m = new HashMap<>();
        String[] ws = text.split(" ");
        for (int i = 0; i < ws.length; i++) m.put(ws[i], m.getOrDefault(ws[i], 0) + 1);
        return m;
    }

    static void eq(double a, double b, String what) {
        if (a != b) throw new AssertionError(what + ": " + a + " != " + b);
    }

    public static void main(String[] args) {
        int N = 1024;
        double[] A = Gauntlet.makeVec(N, 1), B = Gauntlet.makeVec(N, 2), C = Gauntlet.makeVec(N, 9);
        String text = Gauntlet.makeText(2000, 5);

        eq(GenDot.genDot(A, B), dotRef(A, B), "dot");
        eq(GenFilter.genFilter(A), filterRef(A), "filter");
        eq(GenCentroid.genCentroid(A, C), centroidRef(A, C), "centroid");
        eq(GenGeneric.genSumOf(A), sumRef(A), "generic f64");
        HashMap<String,Integer> g = GenWordcount.genWordcount(text), h = wcRef(text);
        if (!g.equals(h)) throw new AssertionError("wordcount");
        if (!GenGeneric.genWordTally(text).equals(h)) throw new AssertionError("generic dict");
        System.out.println("correctness: all five generated programs agree with hand-written\n");

        bench("dot       hand-written", () -> dotRef(A, B));
        bench("dot       GENERATED",    () -> GenDot.genDot(A, B));
        bench("filter    hand-written", () -> filterRef(A));
        bench("filter    GENERATED",    () -> GenFilter.genFilter(A));
        bench("centroid  hand-written", () -> centroidRef(A, C));
        bench("centroid  GENERATED",    () -> GenCentroid.genCentroid(A, C));
    }

    static double sumRef(double[] a) {
        double acc = 0.0;
        for (int i = 0; i < a.length; i++) acc += a[i];
        return acc;
    }

    static double sink;
    interface D { double run(); }
    static void bench(String name, D fn) {
        for (int i = 0; i < 200_000; i++) sink += fn.run();
        double[] ts = new double[9];
        for (int r = 0; r < 9; r++) {
            long t0 = System.nanoTime();
            for (int i = 0; i < 20_000; i++) sink += fn.run();
            ts[r] = (System.nanoTime() - t0) / 20_000.0;
        }
        java.util.Arrays.sort(ts);
        System.out.printf("%-26s %8.1f ns/op%n", name, ts[4]);
    }
}
