/**
 * Re-testing baseline R5, which is what `targets/java/util.oro` declares on.
 *
 * R5 (2026-08-13) measured Java's fused `merge(w, 1, Integer::sum)` at
 * 9,259,530 ns against `put(w, getOrDefault(w,0)+1)` at 3,577,103 — the
 * unfused form winning by 2.6x — and the target file declares the unfused
 * idiom because of it.
 *
 * That number is the basis of ADR 0008's headline example, so it is worth
 * checking rather than inheriting. This warms each form heavily and
 * independently, because a `merge` call site goes through invokedynamic and a
 * functional interface, and those need more warmup than a plain `put` before
 * C2 inlines them — the most likely way to measure this wrong.
 */
public final class MergeCheck {

    public static java.util.Map<String, Integer> unfused(String[] ws) {
        java.util.Map<String, Integer> m = new java.util.HashMap<>();
        for (int i = 0; i < ws.length; i++) m.put(ws[i], m.getOrDefault(ws[i], 0) + 1);
        return m;
    }

    public static java.util.Map<String, Integer> fused(String[] ws) {
        java.util.Map<String, Integer> m = new java.util.HashMap<>();
        for (int i = 0; i < ws.length; i++) m.merge(ws[i], 1, Integer::sum);
        return m;
    }

    static double run(String what, java.util.function.Function<String[], Object> f,
            String[] ws, int warm, int iters) {
        for (int i = 0; i < warm; i++) f.apply(ws);
        double best = Double.MAX_VALUE;
        for (int r = 0; r < 7; r++) {
            long t0 = System.nanoTime();
            for (int i = 0; i < iters; i++) f.apply(ws);
            double d = (System.nanoTime() - t0) / (double) iters;
            if (d < best) best = d;
        }
        System.out.printf("%-28s %12.0f ns%n", what, best);
        return best;
    }

    public static void main(String[] a) {
        String[] ws = Gauntlet.makeText(1 << 16, 5).split(" ");
        if (!unfused(ws).equals(fused(ws))) throw new AssertionError("disagree");
        System.out.println("words: " + ws.length + ", distinct: " + unfused(ws).size());
        // Heavy, independent warmup: 3000 iterations of a ~4 ms operation is
        // about twelve seconds each, far past C2's thresholds and past the
        // point where an invokedynamic call site has been linked and inlined.
        double u = run("unfused put/getOrDefault", MergeCheck::unfused, ws, 3000, 300);
        double f = run("fused merge(Integer::sum)", MergeCheck::fused, ws, 3000, 300);
        System.out.printf("%nfused / unfused = %.2fx   (R5 recorded 2.59x)%n", f / u);

        // R5's EXACT functions, which split inside the timed region.
        String text = Gauntlet.makeText(1 << 16, 5);
        java.util.function.Function<String[], Object> mg =
                z -> Gauntlet.wordCountMerge(text);
        java.util.function.Function<String[], Object> go =
                z -> Gauntlet.wordCountGetOr(text);
        System.out.println();
        System.out.println("R5 own functions (split inside the timed region):");
        double u2 = run("wordCountGetOr", go, ws, 2000, 200);
        double f2 = run("wordCountMerge", mg, ws, 2000, 200);
        System.out.printf("%nmerge / getOr = %.2fx   (R5 recorded 2.59x)%n", f2 / u2);
    }
}
