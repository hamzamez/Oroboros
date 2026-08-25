/** One benchmark per JVM. The in-process harness cannot be trusted on a loaded
 *  machine, and a benchmark run next to nineteen others is not a measurement. */
public final class One {
    static final int N = 1 << 16;
    public static void main(String[] a) {
        double[] A = Gauntlet.makeVec(N, 1), B = Gauntlet.makeVec(N, 2);
        double[] sA = Gauntlet.makeVec(1024, 1), sB = Gauntlet.makeVec(1024, 2);
        java.util.function.Supplier<Object> f = switch (a[0]) {
            case "dot-hand"      -> () -> NativeBench.dotRef(A, B);
            case "dot-long"      -> () -> NativeBench.dotLongRef(A, B);
            case "dot-gen"       -> () -> NatDot.NatDotDot(A, B);
            case "dots-hand"     -> () -> NativeBench.dotRef(sA, sB);
            case "dots-gen"      -> () -> NatDot.NatDotDot(sA, sB);
            case "cent-hand"     -> () -> NativeBench.centroidRef(A, B);
            case "cent-gen"      -> () -> NatCentroid.NatCentroidCentroidX(A, B);
            case "late-hand"     -> () -> NativeBench.findFirstRef(A, 2.0);
            case "late-long"     -> () -> NativeBench.findFirstLongRef(A, 2.0);
            case "late-gen"      -> () -> NatSearch.NatSearchFindFirst(A, 2.0);
            case "sten-hand"     -> () -> NativeBench2.smoothAllocRef(A);
            case "sten-gen"      -> () -> NatSm.NatSmSmoothAlloc(A);
            default -> throw new IllegalArgumentException(a[0]);
        };
        long t = System.nanoTime();
        int w = 0;
        while (System.nanoTime() - t < 2_000_000_000L) { f.get(); w++; }
        int iters = Math.max(50, w / 20);
        double best = Double.MAX_VALUE;
        for (int r = 0; r < 9; r++) {
            long t0 = System.nanoTime();
            for (int i = 0; i < iters; i++) f.get();
            double d = (System.nanoTime() - t0) / (double) iters;
            if (d < best) best = d;
        }
        System.out.printf("%-12s %12.1f ns%n", a[0], best);
    }
}
