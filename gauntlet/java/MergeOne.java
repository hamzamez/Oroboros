/** One form per JVM, because JIT state carries across benchmarks in a process
 *  the way V8's does (native-js-2026-08-20 §1). Pass "fused" or "unfused". */
public final class MergeOne {
    public static void main(String[] a) {
        String[] ws = Gauntlet.makeText(1 << 16, 5).split(" ");
        boolean fused = a[0].equals("fused");
        for (int i = 0; i < 3000; i++) { if (fused) MergeCheck.fused(ws); else MergeCheck.unfused(ws); }
        double best = Double.MAX_VALUE;
        for (int r = 0; r < 7; r++) {
            long t0 = System.nanoTime();
            for (int i = 0; i < 300; i++) { if (fused) MergeCheck.fused(ws); else MergeCheck.unfused(ws); }
            double d = (System.nanoTime() - t0) / 300.0;
            if (d < best) best = d;
        }
        System.out.printf("%-10s %12.0f ns%n", a[0], best);
    }
}
