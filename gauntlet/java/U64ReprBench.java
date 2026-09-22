import java.math.BigInteger;

/**
 * A VALUE IN [0, 2^64) ON THE JVM: which representation -- ADR 0026 (10),
 * u64repr-2026-09-22.
 *
 * The JVM has no unsigned types. The candidates:
 *
 *   ulong    a long holding the value's residue mod 2^64. `v * 256 + b` is
 *            computed with Java's wrapping `*` and `+`, which is LEGAL BY THE
 *            HOMOMORPHISM: Z -> Z/2^64 is a ring map and the result is proven in
 *            [0, 2^64), so the residue names exactly one integer in it. Order
 *            is not preserved by the quotient, so comparison and division go
 *            through Long.compareUnsigned / divideUnsigned / remainderUnsigned;
 *   biginteger  BigInteger, immutable, an allocation per operation;
 *   long     a signed long -- the baseline, and WRONG past 2^63 (check says so).
 *
 * Kernels as in gauntlet/js/u64repr.mjs: `decode` is binary.BigEndian.Uint64 and
 * a running unsigned maximum; `digits` is strconv.FormatUint's divide-by-10 loop.
 *
 *   javac -d out U64ReprBench.java
 *   java -cp out U64ReprBench check
 *   java -cp out U64ReprBench <kernel> <variant> <iters>
 */
public final class U64ReprBench {

    static final int N = 1024;
    static final byte[] bytes = new byte[8 * N];
    static final long[] valsU = new long[N];
    static final BigInteger[] valsB = new BigInteger[N];

    static {
        int seed = 0x9e3779b9;
        for (int i = 0; i < bytes.length; i++) {
            seed = seed * 1103515245 + 12345;
            bytes[i] = (byte) (seed >>> 24);
        }
        for (int k = 0; k < N; k++) {
            long v = 0;
            BigInteger b = BigInteger.ZERO;
            for (int i = 0; i < 8; i++) {
                v = v * 256 + (bytes[8 * k + i] & 0xff);
                b = b.shiftLeft(8).add(BigInteger.valueOf(bytes[8 * k + i] & 0xff));
            }
            valsU[k] = v;
            valsB[k] = b;
        }
    }

    // ------------------------------------------------------------ decode

    static BigInteger decodeULong() {
        long max = 0;
        for (int k = 0; k < N; k++) {
            long v = 0;
            for (int i = 0; i < 8; i++) v = v * 256 + (bytes[8 * k + i] & 0xff);
            if (Long.compareUnsigned(v, max) > 0) max = v;
        }
        return new BigInteger(Long.toUnsignedString(max));
    }

    static BigInteger decodeBig() {
        BigInteger max = BigInteger.ZERO;
        BigInteger k256 = BigInteger.valueOf(256);
        for (int k = 0; k < N; k++) {
            BigInteger v = BigInteger.ZERO;
            for (int i = 0; i < 8; i++) v = v.multiply(k256).add(BigInteger.valueOf(bytes[8 * k + i] & 0xff));
            if (v.compareTo(max) > 0) max = v;
        }
        return max;
    }

    static BigInteger decodeLong() {
        long max = 0;
        for (int k = 0; k < N; k++) {
            long v = 0;
            for (int i = 0; i < 8; i++) v = v * 256 + (bytes[8 * k + i] & 0xff);
            if (v > max) max = v;
        }
        return BigInteger.valueOf(max);
    }

    // ------------------------------------------------------------ digits

    static BigInteger digitsULong() {
        long s = 0;
        for (int k = 0; k < N; k++) {
            long v = valsU[k];
            while (v != 0) {
                s += Long.remainderUnsigned(v, 10);
                v = Long.divideUnsigned(v, 10);
            }
        }
        return BigInteger.valueOf(s);
    }

    static BigInteger digitsBig() {
        long s = 0;
        for (int k = 0; k < N; k++) {
            BigInteger v = valsB[k];
            while (v.signum() != 0) {
                BigInteger[] qr = v.divideAndRemainder(BigInteger.TEN);
                s += qr[1].longValue();
                v = qr[0];
            }
        }
        return BigInteger.valueOf(s);
    }

    static BigInteger digitsLong() {
        long s = 0;
        for (int k = 0; k < N; k++) {
            long v = valsU[k];
            while (v != 0) {
                s += v % 10;
                v /= 10;
            }
        }
        return BigInteger.valueOf(s);
    }

    interface K { BigInteger run(); }

    static K pick(String kernel, String variant) {
        switch (kernel + " " + variant) {
            case "decode ulong": return U64ReprBench::decodeULong;
            case "decode biginteger": return U64ReprBench::decodeBig;
            case "decode long": return U64ReprBench::decodeLong;
            case "digits ulong": return U64ReprBench::digitsULong;
            case "digits biginteger": return U64ReprBench::digitsBig;
            case "digits long": return U64ReprBench::digitsLong;
        }
        throw new IllegalArgumentException(kernel + " " + variant);
    }

    static Object sink;

    public static void main(String[] args) {
        if (args.length == 0 || args[0].equals("check")) {
            int bad = 0;
            for (String kernel : new String[] {"decode", "digits"}) {
                BigInteger want = pick(kernel, "biginteger").run();
                for (String v : new String[] {"biginteger", "ulong", "long"}) {
                    BigInteger got = pick(kernel, v).run();
                    boolean ok = got.equals(want);
                    if (!ok && !v.equals("long")) bad++;
                    System.out.println(kernel + " " + v + ": " + (ok ? "ok" : "WRONG (" + got + " != " + want + ")"));
                }
            }
            System.out.println(bad == 0 ? "every candidate agrees with BigInteger (long is expected to be wrong)" : bad + " FAILURES");
            System.exit(bad);
        }
        K f = pick(args[0], args[1]);
        int iters = Integer.parseInt(args[2]);
        for (int i = 0; i < Math.max(2000, iters / 2); i++) sink = f.run(); // warm past C2
        double[] reps = new double[5];
        for (int r = 0; r < 5; r++) {
            long t0 = System.nanoTime();
            for (int i = 0; i < iters; i++) sink = f.run();
            reps[r] = (double) (System.nanoTime() - t0) / iters / N;
        }
        java.util.Arrays.sort(reps);
        System.out.printf("%s %s ns/value min %.2f median %.2f%n", args[0], args[1], reps[0], reps[2]);
    }
}
