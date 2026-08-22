/** Does C2 still scalar-replace a generated record across a class boundary?
 *
 *  product-2026-08-19 measured a record returned from a hot call at 0.96x —
 *  but the record and the caller were in one file. Here the record is generated
 *  by the compiler in GenDivmod and the caller is this hand-written class,
 *  which is the shape a real program has and the shape nobody measured. */
public final class DivmodBench {

    static long sinkQ, sinkR, sinkS;
    static long divisor = 7;

    record Pair(long f0, long f1) {}

    static Pair divmodRef(long a, long b)  { return new Pair(a / b, a % b); }
    static long divmodSumRef(long a, long b) { return a / b + a % b; }

    static double bench(String name, Runnable fn, int iters) {
        for (int i = 0; i < Math.max(iters * 3, 200_000); i++) fn.run();
        double[] ts = new double[7];
        for (int r = 0; r < 7; r++) {
            long t0 = System.nanoTime();
            for (int i = 0; i < iters; i++) fn.run();
            ts[r] = (System.nanoTime() - t0) / (double) iters;
        }
        java.util.Arrays.sort(ts);
        System.out.printf("%-34s %10.3f ns/op%n", name, ts[3]);
        return ts[3];
    }

    public static void main(String[] args) {
        // agreement first
        for (long a : new long[]{7, -7, 1L << 40}) {
            var g = GenDivmod.nativeDivmod(a, 7);
            var w = divmodRef(a, 7);
            if (g.f0() != w.f0() || g.f1() != w.f1())
                throw new AssertionError("divmod disagrees at " + a);
            if (GenDivmod.nativeDivmodSum(a, 7) != divmodSumRef(a, 7))
                throw new AssertionError("sum disagrees at " + a);
        }
        System.out.println("emitted agrees with hand-written\n");

        final int N = 1000;
        bench("divmod      hand (same class)", () -> {
            for (int i = 0; i < N; i++) { var p = divmodRef(i | 1, divisor); sinkQ = p.f0(); sinkR = p.f1(); }
        }, 2000);
        bench("divmod      NATIVE (other class)", () -> {
            for (int i = 0; i < N; i++) { var p = GenDivmod.nativeDivmod(i | 1, divisor); sinkQ = p.f0(); sinkR = p.f1(); }
        }, 2000);
        bench("divmod-sum  hand (no product)", () -> {
            for (int i = 0; i < N; i++) sinkS = divmodSumRef(i | 1, divisor);
        }, 2000);
        bench("divmod-sum  NATIVE (no product)", () -> {
            for (int i = 0; i < N; i++) sinkS = GenDivmod.nativeDivmodSum(i | 1, divisor);
        }, 2000);
    }
}
