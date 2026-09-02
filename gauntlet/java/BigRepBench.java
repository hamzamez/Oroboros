import java.math.BigInteger;

/**
 * ARBITRARY PRECISION ON THE JVM: what we emit against what a person writes.
 *
 * BigArithBench.java asks whether OUR limb form beats BigInteger, which is the
 * R3 design question and which that file answers emphatically -- ours wins by
 * 6.2x at 50! and still 1.84x at 314 limbs, with no crossover found, because
 * BigInteger is IMMUTABLE and allocates an object and an int[] per operation.
 *
 * This file asks the gauntlet's usual question instead: is the code we emit at
 * parity with the code a Java programmer writes on this host. The reference is
 * therefore the ordinary BigInteger loop and not the limb form -- and saying so
 * is the point, because "at parity" without naming the reference is the shape
 * of error this repository has caught in itself three times.
 *
 * WHAT TO WATCH IS THE COUNTER. `i` is a machine `int` in both the emitted and
 * the hand-written form, and that is rule (P)'s provability gate in
 * emit/bigrep.go doing its job: `i` is read by the big multiply, so a solver
 * that promoted everything a bignum touches would give this loop a BigInteger
 * counter, a BigInteger compare and a BigInteger increment -- three allocations
 * per iteration for a value that provably fits.
 *
 *   javac -d out gen/GenBigFib.java gen/GenBigFact.java gen/GenBigPower.java BigRepBench.java
 *   java -cp out BigRepBench
 */
public final class BigRepBench {

    static Object sink;

    // ------------------------------------------------------------ hand-written

    static BigInteger fibHand(int n) {
        BigInteger a = BigInteger.ZERO, b = BigInteger.ONE;
        for (int i = 0; i < n; i++) {
            BigInteger t = a.add(b);
            a = b;
            b = t;
        }
        return a;
    }

    static BigInteger factHand(int n) {
        BigInteger acc = BigInteger.ONE;
        for (int i = 2; i <= n; i++) acc = acc.multiply(BigInteger.valueOf(i));
        return acc;
    }

    static BigInteger powerHand(int b, int e) {
        BigInteger acc = BigInteger.ONE, x = BigInteger.valueOf(b);
        for (int k = e; k != 0; k /= 2) {
            if (k % 2 == 1) acc = acc.multiply(x);
            x = x.multiply(x);
        }
        return acc;
    }

    // ------------------------------------------------------------ correctness
    //
    // Against BigInteger's own `pow` for the power, which is an oracle rather
    // than the same loop under another name.

    static int check() {
        int bad = 0;
        for (int n : new int[]{0, 1, 2, 10, 50, 90, 100, 200, 300}) {
            if (!GenBigFib.genFib(n).equals(fibHand(n))) {
                System.out.println("FAIL fib(" + n + ")");
                bad++;
            }
        }
        if (!GenBigFib.genFib(100).equals(new BigInteger("354224848179261915075"))) {
            System.out.println("FAIL fib(100) is not the true value");
            bad++;
        }
        for (int n : new int[]{0, 1, 5, 20, 30, 100, 200}) {
            if (!GenBigFact.genFact(n).equals(factHand(n))) {
                System.out.println("FAIL fact(" + n + ")");
                bad++;
            }
        }
        for (int[] c : new int[][]{{2, 10}, {3, 40}, {7, 33}, {999, 64}, {1000, 60}, {5, 0}}) {
            BigInteger want = BigInteger.valueOf(c[0]).pow(c[1]);
            if (!GenBigPower.genPower(c[0], c[1]).equals(want)) {
                System.out.println("FAIL power(" + c[0] + "," + c[1] + ")");
                bad++;
            }
        }
        System.out.println(bad == 0
                ? "ok -- emitted agrees with hand-written and with BigInteger"
                : bad + " FAILURES");
        return bad;
    }

    // ------------------------------------------------------------ timing

    interface Body { void run(); }

    static double bench(Body b, int iters) {
        for (int i = 0; i < Math.max(iters, 2000); i++) b.run();
        double[] t = new double[7];
        for (int r = 0; r < 7; r++) {
            long t0 = System.nanoTime();
            for (int i = 0; i < iters; i++) b.run();
            t[r] = (System.nanoTime() - t0) / (double) iters;
        }
        java.util.Arrays.sort(t);
        return t[3];
    }

    public static void main(String[] args) {
        if (check() != 0) System.exit(1);
        int it = 20000;
        System.out.printf("fib-gen      %9.1f ns/op%n", bench(() -> sink = GenBigFib.genFib(1000), it));
        System.out.printf("fib-hand     %9.1f ns/op%n", bench(() -> sink = fibHand(1000), it));
        System.out.printf("fact-gen     %9.1f ns/op%n", bench(() -> sink = GenBigFact.genFact(200), it));
        System.out.printf("fact-hand    %9.1f ns/op%n", bench(() -> sink = factHand(200), it));
        System.out.printf("power-gen    %9.1f ns/op%n", bench(() -> sink = GenBigPower.genPower(999, 64), it));
        System.out.printf("power-hand   %9.1f ns/op%n", bench(() -> sink = powerHand(999, 64), it));
    }
}
