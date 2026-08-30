import java.math.BigInteger;

/**
 * Karatsuba without recursion, in place, on Java.
 *
 * The port of gauntlet/go/karatsuba2.go. Same shape everywhere: two of the
 * three children are SUBRANGES of the parent, so a node is an offset and a
 * length into one arena rather than a buffer, and the tree is a flat descriptor
 * table filled by a loop. Nothing recurses and nothing is pushed.
 *
 * 64-bit limbs here, because bigarith-2026-08-28 §8c found L64 beats L31 for
 * big x big at equal bit size — narrow limbs mean twice the limbs and four
 * times the inner operations, which swamps multiplyHigh's sign correction.
 *
 *   javac -d out Karatsuba.java && java -cp out Karatsuba
 */
public final class Karatsuba {

    static Object sink;

    // ------------------------------------------------------------- unsigned

    static long umulHigh(long x, long y) {
        return Math.multiplyHigh(x, y) + ((x >> 63) & y) + ((y >> 63) & x);
    }

    /** dst[dOff+off ..] += src[sOff .. sOff+sLen), carrying to the end of dst. */
    static void addAt(long[] dst, int dOff, int dLen, long[] src, int sOff, int sLen, int off) {
        long carry = 0;
        int i = 0;
        for (; i < sLen; i++) {
            long x = dst[dOff + off + i], s = x + src[sOff + i];
            long c1 = Long.compareUnsigned(s, x) < 0 ? 1 : 0;
            long s2 = s + carry;
            long c2 = Long.compareUnsigned(s2, s) < 0 ? 1 : 0;
            dst[dOff + off + i] = s2;
            carry = c1 + c2;
        }
        for (; carry != 0 && off + i < dLen; i++) {
            long x = dst[dOff + off + i], s = x + carry;
            carry = Long.compareUnsigned(s, x) < 0 ? 1 : 0;
            dst[dOff + off + i] = s;
        }
    }

    /** dst[dOff+off ..] -= src, borrowing to the end. The caller guarantees the
     *  running total stays non-negative, so the borrow resolves inside dst. */
    static void subAt(long[] dst, int dOff, int dLen, long[] src, int sOff, int sLen, int off) {
        long borrow = 0;
        int i = 0;
        for (; i < sLen; i++) {
            long x = dst[dOff + off + i], y = src[sOff + i];
            long s = x - y;
            long b1 = Long.compareUnsigned(x, y) < 0 ? 1 : 0;
            long s2 = s - borrow;
            long b2 = Long.compareUnsigned(s, borrow) < 0 ? 1 : 0;
            dst[dOff + off + i] = s2;
            borrow = b1 + b2;
        }
        for (; borrow != 0 && off + i < dLen; i++) {
            long x = dst[dOff + off + i];
            long s = x - borrow;
            borrow = Long.compareUnsigned(x, borrow) < 0 ? 1 : 0;
            dst[dOff + off + i] = s;
        }
    }

    /** Schoolbook, at offsets. out must hold 2n limbs and is zeroed here. */
    static void mulAt(long[] ar, int ao, int bo, int n, long[] out, int po, int oLen) {
        java.util.Arrays.fill(out, po, po + oLen, 0L);
        for (int i = 0; i < n; i++) {
            long carry = 0, ai = ar[ao + i];
            for (int j = 0; j < n; j++) {
                long hi = umulHigh(ai, ar[bo + j]);
                long lo = ai * ar[bo + j];
                long s = lo + carry;
                if (Long.compareUnsigned(s, lo) < 0) hi++;
                long d = out[po + i + j];
                long s2 = d + s;
                if (Long.compareUnsigned(s2, s) < 0) hi++;
                out[po + i + j] = s2;
                carry = hi;
            }
            out[po + i + n] = carry;
        }
    }

    // ------------------------------------------------------------ workspace

    final int n, D;
    final long[] arena, prod;
    final int[] aOff, bOff, ln, pOff, lenOf, prodOf, baseIdx;

    static int pow3(int k) { int p = 1; for (int i = 0; i < k; i++) p *= 3; return p; }

    Karatsuba(int n, int D) {
        this.n = n; this.D = D;
        lenOf = new int[D + 1];
        lenOf[0] = n;
        for (int L = 0; L < D; L++) lenOf[L + 1] = (lenOf[L] - lenOf[L] / 2) + 1;

        baseIdx = new int[D + 2];
        int acc = 0, p = 1;
        for (int L = 0; L <= D; L++) { baseIdx[L] = acc; acc += p; p *= 3; }
        baseIdx[D + 1] = acc;
        int nodes = acc;

        int ar = 2 * n;
        p = 1;
        for (int L = 0; L < D; L++) { ar += p * 2 * lenOf[L + 1]; p *= 3; }
        arena = new long[ar];

        aOff = new int[nodes]; bOff = new int[nodes];
        ln = new int[nodes];  pOff = new int[nodes];

        // Product sizes bottom-up and exact: a parent must reach 2h + a child's.
        prodOf = new int[D + 1];
        prodOf[D] = 2 * lenOf[D];
        for (int L = D - 1; L >= 0; L--) prodOf[L] = 2 * (lenOf[L] / 2) + prodOf[L + 1];

        int tot = 0;
        p = 1;
        for (int L = 0; L <= D; L++) {
            for (int k = 0; k < p; k++) { pOff[baseIdx[L] + k] = tot; tot += prodOf[L]; }
            p *= 3;
        }
        prod = new long[tot];
    }

    long[] mul(long[] a, long[] b) {
        System.arraycopy(a, 0, arena, 0, n);
        System.arraycopy(b, 0, arena, n, n);
        aOff[0] = 0; bOff[0] = n; ln[0] = n;

        int free = 2 * n;
        for (int L = 0; L < D; L++) {
            int base = baseIdx[L], cbase = baseIdx[L + 1], cl = lenOf[L + 1];
            for (int k = 0, p = pow3(L); k < p; k++) {
                int id = base + k, ao = aOff[id], bo = bOff[id], l = ln[id], h = l / 2;
                int c0 = cbase + 3 * k;
                aOff[c0] = ao;     bOff[c0] = bo;     ln[c0] = h;
                aOff[c0 + 1] = ao + h; bOff[c0 + 1] = bo + h; ln[c0 + 1] = l - h;
                int as = free, bs = free + cl;
                free += 2 * cl;
                aOff[c0 + 2] = as; bOff[c0 + 2] = bs; ln[c0 + 2] = cl;
                sumInto(as, cl, ao, h, ao + h, l - h);
                sumInto(bs, cl, bo, h, bo + h, l - h);
            }
        }

        int base = baseIdx[D];
        for (int k = 0, p = pow3(D); k < p; k++) {
            int id = base + k;
            mulAt(arena, aOff[id], bOff[id], ln[id], prod, pOff[id], prodOf[D]);
        }

        for (int L = D - 1; L >= 0; L--) {
            int b0 = baseIdx[L], cbase = baseIdx[L + 1], csz = prodOf[L + 1], sz = prodOf[L];
            for (int k = 0, p = pow3(L); k < p; k++) {
                int id = b0 + k, h = ln[id] / 2, po = pOff[id];
                java.util.Arrays.fill(prod, po, po + sz, 0L);
                int c0 = pOff[cbase + 3 * k];
                addAt(prod, po, sz, prod, c0, csz, 0);
                addAt(prod, po, sz, prod, c0 + csz, csz, 2 * h);
                addAt(prod, po, sz, prod, c0 + 2 * csz, csz, h);
                subAt(prod, po, sz, prod, c0, csz, h);
                subAt(prod, po, sz, prod, c0 + csz, csz, h);
            }
        }
        return prod;
    }

    private void sumInto(int dst, int dLen, int lo, int loLen, int hi, int hiLen) {
        java.util.Arrays.fill(arena, dst, dst + dLen, 0L);
        System.arraycopy(arena, hi, arena, dst, hiLen);
        addAt(arena, dst, dLen, arena, lo, loLen, 0);
    }

    // --------------------------------------------------------------- checks

    static BigInteger toBig(long[] l, int off, int n) {
        BigInteger out = BigInteger.ZERO;
        for (int i = n - 1; i >= 0; i--)
            out = out.shiftLeft(64).or(new BigInteger(Long.toUnsignedString(l[off + i])));
        return out;
    }

    static long[] limbsOf(int n, long seed) {
        long[] out = new long[n];
        long x = seed;
        for (int i = 0; i < n; i++) { x = x * 6364136223846793005L + 1442695040888963407L; out[i] = x | (1L << 63); }
        return out;
    }

    static void run(String what, java.util.function.Supplier<Object> f, int warm, int iters) {
        for (int i = 0; i < warm; i++) sink = f.get();
        double best = Double.MAX_VALUE;
        for (int r = 0; r < 9; r++) {
            long t0 = System.nanoTime();
            for (int i = 0; i < iters; i++) sink = f.get();
            double d = (System.nanoTime() - t0) / (double) iters;
            if (d < best) best = d;
        }
        System.out.printf("%-30s %12.1f ns%n", what, best);
    }

    public static void main(String[] args) {
        for (int n : new int[]{16, 32, 64, 256, 1024}) {
            long[] a = limbsOf(n, 12345), b = limbsOf(n, 67890);
            BigInteger want = toBig(a, 0, n).multiply(toBig(b, 0, n));
            for (int d = 0; d <= 5 && (n >> d) >= 4; d++) {
                Karatsuba w = new Karatsuba(n, d);
                if (!toBig(w.mul(a, b), 0, 2 * n).equals(want))
                    throw new AssertionError("n=" + n + " D=" + d);
            }
        }
        System.out.println("ok — every depth agrees with BigInteger");

        for (int n : new int[]{256, 1024}) {
            long[] a = limbsOf(n, 12345), b = limbsOf(n, 67890);
            BigInteger ba = toBig(a, 0, n), bb = toBig(b, 0, n);
            int w = n >= 1024 ? 2000 : 20000, it = n >= 1024 ? 2000 : 20000;
            System.out.println("-- " + n + " limbs (" + (n * 64) + " bits)");
            run("  BigInteger", () -> ba.multiply(bb), w, it);
            for (int d : new int[]{0, 2, 4, 5, 6, 7, 8}) {
                if ((n >> d) < 4) continue;
                Karatsuba k = new Karatsuba(n, d);
                final int dd = d;
                run("  Karatsuba D=" + dd, () -> k.mul(a, b), w, it);
            }
        }
    }
}
