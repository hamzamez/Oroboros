// Hand-written Java reference implementations.
//
//   G1  dot / dotUnordered4        — cost of left-to-right summation
//   G2  centroidAoS vs centroidSoA — Point[] is an array of references
//   G3  sumMono vs sumFold         — cost of a lambda parameter
//   G4  wordCountMerge / GetOr     — HashMap idioms
//   G6  buildOps / makeScaler      — escaping closures

import java.util.HashMap;
import java.util.Map;
import java.util.function.IntUnaryOperator;

public final class Gauntlet {

    // ------------------------------------------------------------ G1

    public static double dot(double[] xs, double[] ys) {
        double acc = 0.0;
        for (int i = 0; i < xs.length; i++) acc += xs[i] * ys[i];
        return acc;
    }

    public static double dotUnordered4(double[] xs, double[] ys) {
        double a0 = 0, a1 = 0, a2 = 0, a3 = 0;
        int n = xs.length - (xs.length % 4);
        for (int i = 0; i < n; i += 4) {
            a0 += xs[i] * ys[i];
            a1 += xs[i + 1] * ys[i + 1];
            a2 += xs[i + 2] * ys[i + 2];
            a3 += xs[i + 3] * ys[i + 3];
        }
        double acc = (a0 + a1) + (a2 + a3);
        for (int i = n; i < xs.length; i++) acc += xs[i] * ys[i];
        return acc;
    }

    // ------------------------------------------------------------ G2

    public record Point(double x, double y) {}

    // Point[] is an array of references: n separately allocated objects.
    public static double[] centroidAoS(Point[] ps) {
        double accX = 0, accY = 0;
        for (int i = 0; i < ps.length; i++) { accX += ps[i].x(); accY += ps[i].y(); }
        return new double[] { accX / ps.length, accY / ps.length };
    }

    // Parallel double[]: what fast hand-written Java does.
    public static double[] centroidSoA(double[] px, double[] py) {
        double accX = 0, accY = 0;
        for (int i = 0; i < px.length; i++) { accX += px[i]; accY += py[i]; }
        return new double[] { accX / px.length, accY / px.length };
    }

    public static double[] boundsSoA(double[] px, double[] py) {
        double loX = px[0], loY = py[0], hiX = px[0], hiY = py[0];
        for (int i = 1; i < px.length; i++) {
            double x = px[i], y = py[i];
            if (x < loX) loX = x;
            if (y < loY) loY = y;
            if (x > hiX) hiX = x;
            if (y > hiY) hiY = y;
        }
        return new double[] { loX, loY, hiX, hiY };
    }

    // ------------------------------------------------------------ G3

    public interface DoubleFold { double apply(double acc, double x); }

    public static double sumMono(double[] xs) {
        double acc = 0.0;
        for (int i = 0; i < xs.length; i++) acc += xs[i];
        return acc;
    }

    public static double fold(double[] xs, double init, DoubleFold step) {
        double acc = init;
        for (int i = 0; i < xs.length; i++) acc = step.apply(acc, xs[i]);
        return acc;
    }

    public static double sumFold(double[] xs) {
        return fold(xs, 0.0, (a, x) -> a + x);
    }

    // ------------------------------------------------------------ G4

    public static Map<String, Integer> wordCountMerge(String text) {
        Map<String, Integer> counts = new HashMap<>();
        for (String w : text.split(" ")) counts.merge(w, 1, Integer::sum);
        return counts;
    }

    public static Map<String, Integer> wordCountGetOr(String text) {
        Map<String, Integer> counts = new HashMap<>();
        for (String w : text.split(" ")) counts.put(w, counts.getOrDefault(w, 0) + 1);
        return counts;
    }

    // ------------------------------------------------------------ G6

    public static IntUnaryOperator[] buildOps() {
        return new IntUnaryOperator[] { v -> v + 1, v -> v * 2, v -> -v };
    }

    public static int runOp(IntUnaryOperator[] ops, int k, int x) {
        return ops[k].applyAsInt(x);
    }

    public static IntUnaryOperator makeScaler(int f) {
        return v -> v * f;
    }

    // ------------------------------------------------------------ inputs
    // Mulberry32, matching the JS harness bit for bit.

    public static final class Rng {
        private int a;
        Rng(int seed) { this.a = seed; }
        double next() {
            a = a + 0x6d2b79f5;
            int t = a;
            t = (t ^ (t >>> 15)) * (t | 1);
            t ^= t + (t ^ (t >>> 7)) * (t | 61);
            return ((t ^ (t >>> 14)) & 0xFFFFFFFFL) / 4294967296.0;
        }
    }

    public static double[] makeVec(int n, int seed) {
        Rng r = new Rng(seed);
        double[] xs = new double[n];
        for (int i = 0; i < n; i++) xs[i] = r.next() * 2 - 1;
        return xs;
    }

    public static Point[] makePointsAoS(int n, int seed) {
        Rng r = new Rng(seed);
        Point[] ps = new Point[n];
        for (int i = 0; i < n; i++) ps[i] = new Point(r.next() * 2 - 1, r.next() * 2 - 1);
        return ps;
    }

    public static String makeText(int n, int seed) {
        Rng r = new Rng(seed);
        String[] vocab = new String[500];
        for (int i = 0; i < 500; i++) {
            int len = 3 + (int) (r.next() * 6);
            StringBuilder s = new StringBuilder();
            for (int j = 0; j < len; j++) s.append((char) (97 + (int) (r.next() * 26)));
            vocab[i] = s.toString();
        }
        StringBuilder out = new StringBuilder();
        for (int i = 0; i < n; i++) {
            if (i > 0) out.append(' ');
            out.append(vocab[(int) (r.next() * 500)]);
        }
        return out.toString();
    }
}
