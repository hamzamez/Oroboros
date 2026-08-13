import java.util.ArrayList;
import java.util.List;
import java.util.function.IntUnaryOperator;

/** Median-of-runs harness with a warmup long enough to reach C2. */
public final class Bench {

    static final int NVEC = 1 << 16;
    static final int NSMALL = 1024;

    static final List<String> names = new ArrayList<>();
    static final List<Double> nanos = new ArrayList<>();
    static double sink;

    static void bench(String name, Runnable fn, int iters) {
        for (int i = 0; i < Math.max(iters * 3, 50_000); i++) fn.run();
        double[] ts = new double[7];
        for (int r = 0; r < 7; r++) {
            long t0 = System.nanoTime();
            for (int i = 0; i < iters; i++) fn.run();
            ts[r] = (System.nanoTime() - t0) / (double) iters;
        }
        java.util.Arrays.sort(ts);
        names.add(name);
        nanos.add(ts[3]);
    }

    public static void main(String[] args) {
        double[] vecA = Gauntlet.makeVec(NVEC, 1);
        double[] vecB = Gauntlet.makeVec(NVEC, 2);
        double[] smallA = Gauntlet.makeVec(NSMALL, 1);
        double[] smallB = Gauntlet.makeVec(NSMALL, 2);
        Gauntlet.Point[] ptsAoS = Gauntlet.makePointsAoS(NVEC, 4);
        double[] ptsX = Gauntlet.makeVec(NVEC, 41);
        double[] ptsY = Gauntlet.makeVec(NVEC, 42);
        String text = Gauntlet.makeText(NVEC, 5);

        bench("G1 dot             n=65536", () -> sink = Gauntlet.dot(vecA, vecB), 500);
        bench("G1 dot             n=1024 ", () -> sink = Gauntlet.dot(smallA, smallB), 50000);
        bench("G1 dotUnordered4   n=1024 ", () -> sink = Gauntlet.dotUnordered4(smallA, smallB), 50000);

        bench("G2 centroidAoS     n=65536", () -> sink = Gauntlet.centroidAoS(ptsAoS)[0], 500);
        bench("G2 centroidSoA     n=65536", () -> sink = Gauntlet.centroidSoA(ptsX, ptsY)[0], 500);
        bench("G2 boundsSoA       n=65536", () -> sink = Gauntlet.boundsSoA(ptsX, ptsY)[0], 500);

        bench("G3 sumMono         n=1024 ", () -> sink = Gauntlet.sumMono(smallA), 50000);
        bench("G3 sumFold         n=1024 ", () -> sink = Gauntlet.sumFold(smallA), 50000);

        bench("G4 wordCountMerge  n=65536", () -> sink = Gauntlet.wordCountMerge(text).size(), 30);
        bench("G4 wordCountGetOr  n=65536", () -> sink = Gauntlet.wordCountGetOr(text).size(), 30);

        IntUnaryOperator[] ops = Gauntlet.buildOps();
        final int[] k = { 0 };
        bench("G6 buildOps              ", () -> sink = Gauntlet.buildOps().length, 200000);
        bench("G6 runOp                 ", () -> sink = Gauntlet.runOp(ops, (k[0]++) % 3, 7), 500000);
        bench("G6 makeScaler            ", () -> sink = Gauntlet.makeScaler(k[0]++).applyAsInt(3), 200000);

        int w = names.stream().mapToInt(String::length).max().orElse(0);
        for (int i = 0; i < names.size(); i++) {
            System.out.printf("%-" + w + "s  %12.2f ns/op%n", names.get(i), nanos.get(i));
        }
    }
}
