/**
 * THE GAUNTLET'S PAIRS, INTERLEAVED (CLAUDE.md's measurements' method,
 * jsontok-2026-11-19 §3 (b)). A process slows with its age, so the generated
 * form and its reference are timed back to back each round, the order
 * alternating (ABBA cancels a slow drift's first-order term), and the pair's
 * figure is the median of its rounds' ratios.
 *
 * Each form is timed by a method of its own, loopA or loopB, so each call site
 * of Runnable.run sees one target: one shared loop over two lambdas is the
 * megamorphic call site gauntlet-2026-09-29 found.
 *
 * A harness registers its cases with {@link #register} when {@link #on} is set
 * (its main's first argument "--pair"), and ends with {@link #run}:
 *
 *   java -cp out NativeBench --pair "dot  n=65536  GENERATED" "dot  n=65536  hand-written"
 *   java -cp out Pair                 the harness's own test
 */
public final class Pair {
    static boolean on;
    static final java.util.Map<String, Runnable> cases = new java.util.LinkedHashMap<>();
    static int rounds = 7;
    static long batchNs = 20_000_000L;

    /** Reads "--pair" from args, and "--rounds k" after it; answers the rest. */
    static String[] parse(String[] args) {
        if (args.length == 0 || !args[0].equals("--pair")) return args;
        on = true;
        java.util.List<String> names = new java.util.ArrayList<>();
        for (int i = 1; i < args.length; i++) {
            if (args[i].equals("--rounds")) rounds = Integer.parseInt(args[++i]);
            else names.add(args[i]);
        }
        return names.toArray(new String[0]);
    }

    static void register(String name, Runnable fn) { cases.put(name, fn); }

    static double loopA(Runnable fn, int n) {
        long t0 = System.nanoTime();
        for (int i = 0; i < n; i++) fn.run();
        return (System.nanoTime() - t0) / (double) n;
    }

    static double loopB(Runnable fn, int n) {
        long t0 = System.nanoTime();
        for (int i = 0; i < n; i++) fn.run();
        return (System.nanoTime() - t0) / (double) n;
    }

    static double median(double[] xs) {
        double[] s = xs.clone();
        java.util.Arrays.sort(s);
        return s.length % 2 == 1 ? s[s.length / 2] : (s[s.length / 2 - 1] + s[s.length / 2]) / 2;
    }

    /** gen/hand: {n, genNs, handNs, ratio, lo, hi}. */
    static double[] interleave(Runnable gen, Runnable hand, int rounds, long batchNs) {
        // warm each past C2's thresholds: one second or 200,000 calls
        for (int f = 0; f < 2; f++) {
            long w0 = System.nanoTime();
            int w = 0;
            while (System.nanoTime() - w0 < 1_000_000_000L && w < 200) {
                if (f == 0) loopA(gen, 1000); else loopB(hand, 1000);
                w++;
            }
        }
        double per = loopB(hand, 1000);
        int n = (int) Math.max(1, Math.round(batchNs / per));
        double[] gs = new double[rounds], hs = new double[rounds], rs = new double[rounds];
        for (int r = 0; r < rounds; r++) {
            if (r % 2 == 0) { gs[r] = loopA(gen, n); hs[r] = loopB(hand, n); }
            else { hs[r] = loopB(hand, n); gs[r] = loopA(gen, n); }
            rs[r] = gs[r] / hs[r];
        }
        double[] s = rs.clone();
        java.util.Arrays.sort(s);
        return new double[] {n, median(gs), median(hs), median(rs), s[0], s[s.length - 1]};
    }

    /** Measures each named pair, gen then hand, from the registered cases. */
    static void run(String[] names) {
        if (names.length % 2 != 0) throw new IllegalArgumentException("--pair takes gen hand names in pairs");
        for (int i = 0; i < names.length; i += 2) {
            Runnable g = cases.get(names[i]), h = cases.get(names[i + 1]);
            if (g == null || h == null)
                throw new IllegalArgumentException("no such case: " + (g == null ? names[i] : names[i + 1]));
            double[] r = interleave(g, h, rounds, batchNs);
            System.out.printf("%-34s %-34s %8d %12.1f %12.1f %7.3f %.3f-%.3f pair%n",
                    names[i].trim(), names[i + 1].trim(), (long) r[0], r[1], r[2], r[3], r[4], r[5]);
        }
    }

    static double sink;

    static double dot(double[] x, double[] y) {
        double s = 0;
        for (int i = 0; i < x.length; i++) s += x[i] * y[i];
        return s;
    }

    /** THE HARNESS CAN FAIL: twice the work reads about 2, a form against itself about 1. */
    public static void main(String[] args) {
        double[] a = new double[1024], b = new double[1024];
        for (int i = 0; i < a.length; i++) { a[i] = i * 0.5; b[i] = i * 0.25; }
        Runnable one = () -> sink += dot(a, b);
        Runnable two = () -> { sink += dot(a, b); sink += dot(b, a); };
        double r2 = interleave(two, one, 5, 10_000_000L)[3];
        double r1 = interleave(one, one, 5, 10_000_000L)[3];
        boolean ok = r2 > 1.5 && r2 < 2.6 && r1 > 0.85 && r1 < 1.18;
        System.out.printf("twice the work %.3f, itself %.3f: %s%n", r2, r1, ok ? "ok" : "FAIL");
        if (!ok) System.exit(1);
    }
}
