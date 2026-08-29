import java.math.BigInteger;

/**
 * R3 on Java: BigInteger against limb forms we could emit.
 *
 * Third host, after gauntlet/go/bigarith.go (ours wins everywhere, crossing to
 * parity at ~1900 limbs) and gauntlet/js/bigarith.mjs (ours wins to ~100 limbs,
 * V8's BigInt wins past it). Both said the answer is a THRESHOLD rather than a
 * winner, and this host decides whether that generalises.
 *
 * Java is the one expected to favour ours most strongly, for a reason that is
 * about the API rather than the arithmetic: BigInteger is IMMUTABLE, so every
 * operation allocates a fresh object and a fresh int[]. There is a mutable
 * version inside the JDK -- java.math.MutableBigInteger -- and it is
 * package-private, so a caller cannot reach the thing the implementation uses
 * on itself.
 *
 * THREE LIMB WIDTHS, because JS taught that width is the axis that decides it:
 *
 *   L64  64-bit limbs in long[]. Needs the high half of a 64x64 product.
 *        Math.multiplyHigh is a JDK 9 intrinsic, but it is SIGNED, and JDK 17
 *        has no unsignedMultiplyHigh (that is JDK 18), so the correction is
 *        applied by hand below.
 *   L32  32-bit limbs in long[]. A limb times a small multiplier fits in a
 *        long with room to spare, so no high-multiply is needed at all -- the
 *        carry is a shift and a mask. This is what a target with no
 *        multiplyHigh would emit.
 *   L32i 32-bit limbs in int[]. Same arithmetic, half the memory. Our `int` is
 *        a `long` on this host (targets/java), so this is the shape we would
 *        NOT get for free and is measured to price that.
 *
 * BigInteger is the correctness oracle, checked at every size before anything
 * is timed.
 *
 *   javac -d out BigArithBench.java
 *   java -cp out BigArithBench
 */
public final class BigArithBench {

    static Object sink;

    // ------------------------------------------------------------ BigInteger

    static BigInteger factBig(int n) {
        BigInteger acc = BigInteger.ONE;
        for (int k = 2; k <= n; k++) acc = acc.multiply(BigInteger.valueOf(k));
        return acc;
    }

    static BigInteger fibBig(int n) {
        BigInteger a = BigInteger.ZERO, b = BigInteger.ONE;
        for (int i = 0; i < n; i++) { BigInteger t = a.add(b); a = b; b = t; }
        return a;
    }

    // ------------------------------------------------- 64-bit limbs, long[]

    /**
     * The high 64 bits of an UNSIGNED 64x64 product. Math.multiplyHigh is
     * signed; the correction adds back the terms the sign bits dropped. JDK 18
     * has this as Math.unsignedMultiplyHigh and JDK 17 does not.
     */
    static long umulHigh(long x, long y) {
        return Math.multiplyHigh(x, y) + ((x >> 63) & y) + ((y >> 63) & x);
    }

    static int factL64(int n, long[] acc) {
        java.util.Arrays.fill(acc, 0L);
        acc[0] = 1;
        int used = 1;
        for (long k = 2; k <= n; k++) {
            long carry = 0;
            for (int i = 0; i < used; i++) {
                long hi = umulHigh(acc[i], k);
                long lo = acc[i] * k;
                long s = lo + carry;
                // Unsigned overflow of lo+carry, without unsigned types.
                if (Long.compareUnsigned(s, lo) < 0) hi++;
                acc[i] = s;
                carry = hi;
            }
            if (carry != 0) acc[used++] = carry;
        }
        return used;
    }

    static int fibL64(int n, long[][] bufs) {
        long[] a = bufs[0], b = bufs[1], t = bufs[2];
        java.util.Arrays.fill(a, 0L); java.util.Arrays.fill(b, 0L); java.util.Arrays.fill(t, 0L);
        b[0] = 1;
        int ua = 1, ub = 1;
        for (int i = 0; i < n; i++) {
            int u = Math.max(ua, ub);
            long carry = 0;
            for (int j = 0; j < u; j++) {
                long s = a[j] + b[j];
                long c1 = Long.compareUnsigned(s, a[j]) < 0 ? 1 : 0;
                long s2 = s + carry;
                long c2 = Long.compareUnsigned(s2, s) < 0 ? 1 : 0;
                t[j] = s2;
                carry = c1 + c2;
            }
            int ut = u;
            if (carry != 0) { t[u] = carry; ut = u + 1; }
            long[] x = a; a = b; b = t; t = x;
            ua = ub; ub = ut;
        }
        bufs[0] = a; bufs[1] = b; bufs[2] = t;
        return ua;
    }

    // ------------------------------------------------- 32-bit limbs, long[]

    static final long M32 = 0xffffffffL;

    static int factL32(int n, long[] acc) {
        java.util.Arrays.fill(acc, 0L);
        acc[0] = 1;
        int used = 1;
        for (long k = 2; k <= n; k++) {
            long carry = 0;
            for (int i = 0; i < used; i++) {
                long t = acc[i] * k + carry;   // < 2^32 * 2^11, nowhere near overflow
                acc[i] = t & M32;
                carry = t >>> 32;
            }
            while (carry != 0) { acc[used++] = carry & M32; carry = carry >>> 32; }
        }
        return used;
    }

    static int fibL32(int n, long[][] bufs) {
        long[] a = bufs[0], b = bufs[1], t = bufs[2];
        java.util.Arrays.fill(a, 0L); java.util.Arrays.fill(b, 0L); java.util.Arrays.fill(t, 0L);
        b[0] = 1;
        int ua = 1, ub = 1;
        for (int i = 0; i < n; i++) {
            int u = Math.max(ua, ub);
            long carry = 0;
            for (int j = 0; j < u; j++) {
                long s = a[j] + b[j] + carry;
                t[j] = s & M32;
                carry = s >>> 32;
            }
            int ut = u;
            if (carry != 0) { t[u] = carry; ut = u + 1; }
            long[] x = a; a = b; b = t; t = x;
            ua = ub; ub = ut;
        }
        bufs[0] = a; bufs[1] = b; bufs[2] = t;
        return ua;
    }

    // -------------------------------------------------- 32-bit limbs, int[]

    static int factL32i(int n, int[] acc) {
        java.util.Arrays.fill(acc, 0);
        acc[0] = 1;
        int used = 1;
        for (long k = 2; k <= n; k++) {
            long carry = 0;
            for (int i = 0; i < used; i++) {
                long t = (acc[i] & M32) * k + carry;
                acc[i] = (int) t;
                carry = t >>> 32;
            }
            while (carry != 0) { acc[used++] = (int) carry; carry = carry >>> 32; }
        }
        return used;
    }

    // ------------------------------------------------------------ big x big
    //
    // The case the other workloads avoid: both operands large. Factorial
    // multiplies big by SMALL and fibonacci adds, both linear. A big x big
    // product is quadratic for schoolbook and O(n^1.585) for Karatsuba, which
    // BigInteger switches to at 80 ints, with Toom-Cook above 240.
    //
    // TWO BASES, because 32-bit limbs DO NOT WORK here: (2^32-1)^2 exceeds a
    // signed long before anything is accumulated into it. 31-bit limbs leave
    // room for the running sum; 64-bit limbs need multiplyHigh and its sign
    // correction, which §6a found was not worth it for big x small.
    static final long M31 = 0x7fffffffL;

    static void mulL31(long[] a, long[] b, long[] out) {
        java.util.Arrays.fill(out, 0L);
        for (int i = 0; i < a.length; i++) {
            long carry = 0, ai = a[i];
            for (int j = 0; j < b.length; j++) {
                long t = ai * b[j] + out[i + j] + carry;
                out[i + j] = t & M31;
                carry = t >>> 31;
            }
            out[i + b.length] = carry;
        }
    }

    static void mulL64(long[] a, long[] b, long[] out) {
        java.util.Arrays.fill(out, 0L);
        for (int i = 0; i < a.length; i++) {
            long carry = 0, ai = a[i];
            for (int j = 0; j < b.length; j++) {
                long hi = umulHigh(ai, b[j]);
                long lo = ai * b[j];
                long s = lo + carry;
                if (Long.compareUnsigned(s, lo) < 0) hi++;
                long s2 = out[i + j] + s;
                if (Long.compareUnsigned(s2, s) < 0) hi++;
                out[i + j] = s2;
                carry = hi;
            }
            out[i + b.length] = carry;
        }
    }

    static long[] limbsOf(int n, long seed, long mask) {
        long[] out = new long[n];
        long x = seed | (1L << 62);
        for (int i = 0; i < n; i++) {
            x = x * 6364136223846793005L + 1442695040888963407L;
            out[i] = (x | (1L << 62)) & mask;
        }
        return out;
    }

    static BigInteger fromLimbs(long[] l, int w) {
        BigInteger out = BigInteger.ZERO;
        for (int i = l.length - 1; i >= 0; i--) {
            out = out.shiftLeft(w);
            BigInteger limb = w == 64 ? new BigInteger(Long.toUnsignedString(l[i]))
                                      : BigInteger.valueOf(l[i]);
            out = out.or(limb);
        }
        return out;
    }

    // ------------------------------------------------------------- sizing

    static int factBits(int n) {
        int b = 1;
        for (int k = 2; k <= n; k++) b += 64 - Long.numberOfLeadingZeros(k);
        return b;
    }
    static int fibBits(int n) { return (int) Math.ceil(n * 0.695) + 2; }
    static int limbs(int bits, int w) { return (bits + w - 1) / w + 2; }

    // ------------------------------------------------------------ checking

    static BigInteger toBig(long[] l, int used, int w) {
        BigInteger out = BigInteger.ZERO;
        for (int i = used - 1; i >= 0; i--) {
            out = out.shiftLeft(w);
            BigInteger limb = w == 64
                    ? new BigInteger(Long.toUnsignedString(l[i]))
                    : BigInteger.valueOf(l[i] & M32);
            out = out.or(limb);
        }
        return out;
    }

    static BigInteger toBigI(int[] l, int used) {
        BigInteger out = BigInteger.ZERO;
        for (int i = used - 1; i >= 0; i--) out = out.shiftLeft(32).or(BigInteger.valueOf(l[i] & M32));
        return out;
    }

    static long[][] three(int c) { return new long[][]{new long[c], new long[c], new long[c]}; }

    static void check() {
        for (int n : new int[]{0, 1, 2, 5, 20, 21, 50, 200, 2000}) {
            BigInteger want = factBig(n);
            long[] a64 = new long[limbs(factBits(n), 64)];
            if (!toBig(a64, factL64(n, a64), 64).equals(want))
                throw new AssertionError("fact L64 " + n);
            long[] a32 = new long[limbs(factBits(n), 32)];
            if (!toBig(a32, factL32(n, a32), 32).equals(want))
                throw new AssertionError("fact L32 " + n);
            int[] ai = new int[limbs(factBits(n), 32)];
            if (!toBigI(ai, factL32i(n, ai)).equals(want))
                throw new AssertionError("fact L32i " + n);
        }
        for (int n : new int[]{0, 1, 2, 10, 93, 94, 300, 1000}) {
            BigInteger want = fibBig(n);
            // TWO STATEMENTS, not one. Java evaluates arguments left to right,
            // so `toBig(b64[0], fibL64(n, b64), 64)` reads the buffer BEFORE
            // the call rotates it — which is how the first version of this
            // check failed at n=1 on a correct implementation.
            long[][] b64 = three(limbs(fibBits(n), 64));
            int u64 = fibL64(n, b64);
            if (!toBig(b64[0], u64, 64).equals(want))
                throw new AssertionError("fib L64 " + n);
            long[][] b32 = three(limbs(fibBits(n), 32));
            int u32 = fibL32(n, b32);
            if (!toBig(b32[0], u32, 32).equals(want))
                throw new AssertionError("fib L32 " + n);
        }
        // The workload must actually leave the window, or this measures machine
        // arithmetic with extra steps. 21! and fib(93) are the first past 2^63.
        if (factBig(20).bitLength() > 63 || factBig(21).bitLength() <= 63)
            throw new AssertionError("21! crossover");
        if (fibBig(92).bitLength() > 63 || fibBig(93).bitLength() <= 63)
            throw new AssertionError("fib(93) crossover");
        for (int n : new int[]{1, 2, 4, 16, 79, 80, 81, 256}) {
            long[] a31 = limbsOf(n, 12345, M31), b31 = limbsOf(n, 67890, M31);
            long[] o31 = new long[2 * n + 1];
            mulL31(a31, b31, o31);
            if (!fromLimbs(o31, 31).equals(fromLimbs(a31, 31).multiply(fromLimbs(b31, 31))))
                throw new AssertionError("mulL31 " + n);
            long[] a64 = limbsOf(n, 12345, -1L), b64 = limbsOf(n, 67890, -1L);
            long[] o64 = new long[2 * n + 1];
            mulL64(a64, b64, o64);
            if (!fromLimbs(o64, 64).equals(fromLimbs(a64, 64).multiply(fromLimbs(b64, 64))))
                throw new AssertionError("mulL64 " + n);
        }
        System.out.println("ok — every limb form agrees with BigInteger");
    }

    // ------------------------------------------------------------ harness

    static void run(String what, java.util.function.Supplier<Object> f, int warm, int iters) {
        for (int i = 0; i < warm; i++) sink = f.get();
        double best = Double.MAX_VALUE;
        for (int r = 0; r < 9; r++) {
            long t0 = System.nanoTime();
            for (int i = 0; i < iters; i++) sink = f.get();
            double d = (System.nanoTime() - t0) / (double) iters;
            if (d < best) best = d;
        }
        System.out.printf("%-34s %12.1f ns%n", what, best);
    }

    public static void main(String[] args) {
        check();
        for (int n : new int[]{50, 200, 2000}) {
            int w = n == 2000 ? 200 : 20000, it = n == 2000 ? 300 : 5000;
            long[] a64 = new long[limbs(factBits(n), 64)];
            long[] a32 = new long[limbs(factBits(n), 32)];
            int[] ai = new int[limbs(factBits(n), 32)];
            System.out.println("-- " + n + "!  (" + limbs(factBits(n), 64) + " 64-bit limbs)");
            run("  BigInteger", () -> factBig(n), w, it);
            run("  L64 long[]", () -> factL64(n, a64), w, it);
            run("  L32 long[]", () -> factL32(n, a32), w, it);
            run("  L32 int[] ", () -> factL32i(n, ai), w, it);
        }
        // PARAMETERISED BY BITS, not by limbs. The first version sized all three
        // forms at the same LIMB COUNT, so the 31-bit form was multiplying
        // numbers half the size of the 64-bit one and its apparent win was an
        // artefact of measuring less work.
        for (int bits : new int[]{256, 512, 1024, 4096, 16384}) {
            int n64 = (bits + 63) / 64, n31 = (bits + 30) / 31;
            long[] a31 = limbsOf(n31, 12345, M31), b31 = limbsOf(n31, 67890, M31);
            long[] o31 = new long[2 * n31 + 1];
            long[] a64 = limbsOf(n64, 12345, -1L), b64x = limbsOf(n64, 67890, -1L);
            long[] o64 = new long[2 * n64 + 1];
            BigInteger ba = fromLimbs(a64, 64), bb = fromLimbs(b64x, 64);
            int w2 = bits >= 4096 ? 2000 : 20000, i2 = bits >= 4096 ? 2000 : 20000;
            System.out.println("-- big x big, " + bits + " bits (" + n64 + " x 64, " + n31 + " x 31)");
            run("  BigInteger", () -> ba.multiply(bb), w2, i2);
            run("  L64 long[]", () -> { mulL64(a64, b64x, o64); return o64; }, w2, i2);
            run("  L31 long[]", () -> { mulL31(a31, b31, o31); return o31; }, w2, i2);
        }

        int n = 1000;
        long[][] b64 = three(limbs(fibBits(n), 64));
        long[][] b32 = three(limbs(fibBits(n), 32));
        System.out.println("-- fib(" + n + ")  (" + limbs(fibBits(n), 64) + " 64-bit limbs)");
        run("  BigInteger", () -> fibBig(n), 20000, 3000);
        run("  L64 long[]", () -> fibL64(n, b64), 20000, 3000);
        run("  L32 long[]", () -> fibL32(n, b32), 20000, 3000);
    }
}
