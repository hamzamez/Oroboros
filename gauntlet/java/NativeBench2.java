/**
 * The remaining three gauntlet programs on the NATIVE Java target: generic,
 * report and the stencil. One form per JVM where the comparison is close,
 * because JIT state carries across benchmarks in a process.
 *
 * The stencil carries THREE forms, because ADR 0013's question is exactly the
 * difference between them:
 *   alloc  the GATHER -- (alloc (table ...)), pure and parallel
 *   build  the SCATTER -- build/set, sequential, still allocating
 *   into   target-native, writing into a buffer the CALLER owns
 * The third is what the portable language cannot express: ADR 0018 scopes a
 * buffer to `build`, so reusing a caller's array is java.set-double.
 */
public final class NativeBench2 {

    static final int N = 1 << 16;
    static Object sink;

    // ---- hand-written references -------------------------------------------

    static double sumRef(double[] a) {
        double acc = 0.0;
        for (int i = 0; i < a.length; i++) acc += a[i];
        return acc;
    }

    static double[] smoothAllocRef(double[] a) {
        double[] out = new double[a.length - 2];
        for (int j = 0; j < out.length; j++) out[j] = (a[j] + a[j + 1] + a[j + 2]) / 3.0;
        return out;
    }

    static double[] smoothIntoRef(double[] dst, double[] a) {
        for (int j = 0; j < a.length - 2; j++) dst[j] = (a[j] + a[j + 1] + a[j + 2]) / 3.0;
        return dst;
    }

    static double run(String what, java.util.function.Supplier<Object> f, int warm, int iters) {
        for (int i = 0; i < warm; i++) sink = f.get();
        double best = Double.MAX_VALUE;
        for (int r = 0; r < 9; r++) {
            long t0 = System.nanoTime();
            for (int i = 0; i < iters; i++) sink = f.get();
            double d = (System.nanoTime() - t0) / (double) iters;
            if (d < best) best = d;
        }
        System.out.printf("%-36s %10.1f ns%n", what, best);
        return best;
    }

    public static void main(String[] args) {
        double[] A = Gauntlet.makeVec(N, 1);
        double[] dst = new double[N];
        String text = Gauntlet.makeText(N, 5);

        // Correctness before timing.
        if (Math.abs(NatGen.NatGenSumOf(A) - sumRef(A)) > 1e-9)
            throw new AssertionError("generic sum");
        if (!NatGen.NatGenWordTally(text).equals(NatWc.NatWcTallyMerge(text)))
            throw new AssertionError("generic dict");
        double[] r1 = NatSm.NatSmSmoothAlloc(A), r2 = smoothAllocRef(A);
        double[] r3 = NatSm.NatSmSmoothBuild(A);
        if (!java.util.Arrays.equals(r1, r2)) throw new AssertionError("stencil alloc");
        if (!java.util.Arrays.equals(r3, r2)) throw new AssertionError("stencil build");
        NatSm.NatSmSmoothInto(dst, A);
        for (int i = 0; i < A.length - 2; i++)
            if (Math.abs(dst[i] - r2[i]) > 1e-12) throw new AssertionError("stencil into");
        System.out.println("agreement: ok\n");

        System.out.println("-- generic: ONE definition, two element types --");
        run("sum-of      hand-written", () -> sumRef(A), 20000, 2000);
        run("sum-of      GENERATED", () -> NatGen.NatGenSumOf(A), 20000, 2000);
        System.out.println();

        System.out.println("-- stencil (ADR 0013) --");
        run("alloc       hand-written", () -> smoothAllocRef(A), 3000, 500);
        run("alloc       GENERATED", () -> NatSm.NatSmSmoothAlloc(A), 3000, 500);
        run("build       GENERATED", () -> NatSm.NatSmSmoothBuild(A), 3000, 500);
        System.out.println();
        run("into (reuse) hand-written", () -> smoothIntoRef(dst, A), 3000, 500);
        run("into (reuse) GENERATED", () -> NatSm.NatSmSmoothInto(dst, A), 3000, 500);
    }
}
