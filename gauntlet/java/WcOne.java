/** One wordcount form per JVM. Both hand-written forms and both generated. */
public final class WcOne {
    public static void main(String[] a) {
        String text = Gauntlet.makeText(1 << 16, 5);
        java.util.function.Supplier<Object> f = switch (a[0]) {
            case "hand-unfused" -> () -> MergeCheck.unfused(text.split(" "));
            case "hand-fused"   -> () -> MergeCheck.fused(text.split(" "));
            case "gen-unfused"  -> () -> NatWc.NatWcTally(text);
            case "gen-fused"    -> () -> NatWc.NatWcTallyMerge(text);
            default -> throw new IllegalArgumentException(a[0]);
        };
        for (int i = 0; i < 2000; i++) f.get();
        double best = Double.MAX_VALUE;
        for (int r = 0; r < 7; r++) {
            long t0 = System.nanoTime();
            for (int i = 0; i < 200; i++) f.get();
            double d = (System.nanoTime() - t0) / 200.0;
            if (d < best) best = d;
        }
        System.out.printf("%-14s %12.0f ns%n", a[0], best);
    }
}
