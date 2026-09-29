/** One benchmark per JVM. The in-process harness cannot be trusted on a loaded
 *  machine, and a benchmark run next to nineteen others is not a measurement. */
public final class One {
    static final int N = 1 << 16;
    public static void main(String[] a) {
        double[] A = Gauntlet.makeVec(N, 1), B = Gauntlet.makeVec(N, 2);
        double[] sA = Gauntlet.makeVec(1024, 1), sB = Gauntlet.makeVec(1024, 2);
        double[] dst = new double[N];
        String text = Gauntlet.makeText(N, 5);
        long[] tree = JsonTreeBench.longsOf(JsonTreeBench.makeDoc(20));
        short[] treeS = JsonTreeBench.shortsOf(JsonTreeBench.makeDoc(20));
        short[] tokS = JsonTokBench.shortsOf(JsonTokBench.makeDoc(64));
        java.util.function.Supplier<Object> f = switch (a[0]) {
            case "dot-hand"      -> () -> NativeBench.dotRef(A, B);
            case "dot-long"      -> () -> NativeBench.dotLongRef(A, B);
            case "dot-gen"       -> () -> NatDot.natDotDot(A, B);
            case "dots-hand"     -> () -> NativeBench.dotRef(sA, sB);
            case "dots-gen"      -> () -> NatDot.natDotDot(sA, sB);
            case "cent-hand"     -> () -> NativeBench.centroidRef(A, B);
            case "cent-gen"      -> () -> NatCentroid.natCentroidCentroidX(A, B);
            case "late-hand"     -> () -> NativeBench.findFirstRef(A, 2.0);
            case "late-long"     -> () -> NativeBench.findFirstLongRef(A, 2.0);
            case "late-gen"      -> () -> NatSearch.natSearchFindFirst(A, 2.0);
            case "sten-hand"     -> () -> NativeBench2.smoothAllocRef(A);
            case "sten-gen"      -> () -> NatSm.natSmSmoothAlloc(A);
            case "early-hand"    -> () -> NativeBench.findFirstRef(A, 0.5);
            case "early-gen"     -> () -> NatSearch.natSearchFindFirst(A, 0.5);
            case "wc-hand"       -> () -> NativeBench.wcRef(text);
            case "wc-gen"        -> () -> NatWc.natWcTally(text);
            case "gtally-gen"    -> () -> NatGen.natGenWordTally(text);
            case "sum-hand"      -> () -> NativeBench2.sumRef(A);
            case "sum-gen"       -> () -> NatGen.natGenSumOf(A);
            case "build-gen"     -> () -> NatSm.natSmSmoothBuild(A);
            case "into-hand"     -> () -> NativeBench2.smoothIntoRef(dst, A);
            case "into-gen"      -> () -> NatSm.natSmSmoothInto(dst, A);
            case "tree-hand"     -> () -> JsonTreeBench.treeFlat(tree);
            case "tree-gen"      -> () -> GenJsonTree.genMeasure(treeS);
            case "tok-hand"      -> () -> JsonTokBench.tokShorts(tokS);
            case "tok-gen"       -> () -> GenJsonTok.genTokens(tokS);
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
