public class Alias {
    static void smooth(double[] dst, double[] src) {
        for (int i = 1; i < src.length - 1; i++)
            dst[i] = (src[i-1] + src[i] + src[i+1]) / 3;
    }
    static void smoothNoAlias(double[] dst, double[] src) {
        double a = src[0], b = src[1];
        for (int i = 1; i < src.length - 1; i++) {
            double c = src[i+1];
            dst[i] = (a + b + c) / 3;
            a = b; b = c;
        }
    }
    static void bench(String name, Runnable fn, int iters) {
        for (int i = 0; i < iters * 3; i++) fn.run();
        double[] ts = new double[7];
        for (int r = 0; r < 7; r++) {
            long t0 = System.nanoTime();
            for (int i = 0; i < iters; i++) fn.run();
            ts[r] = (System.nanoTime() - t0) / (double) iters;
        }
        java.util.Arrays.sort(ts);
        System.out.printf("%-30s %10.0f ns/op%n", name, ts[3]);
    }
    public static void main(String[] a) {
        int N = 1 << 16;
        double[] src = Gauntlet.makeVec(N, 7);
        double[] dst = new double[N];
        bench("smooth disjoint", () -> smooth(dst, src), 300);
        bench("smoothNoAlias disjoint", () -> smoothNoAlias(dst, src), 300);
        bench("smooth IN-PLACE (aliased)", () -> smooth(src, src), 300);
        bench("smoothFresh (allocates)", () -> smooth(new double[N], src), 300);
    }
}
