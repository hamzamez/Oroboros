/**
 * A JSON tokeniser on Java: emitted against hand-written.
 *
 * The first BRANCHY program in the gauntlet. Everything measured on this host
 * before is a numeric loop over a double[] or a HashMap tally -- countable,
 * predictable, array-walking. A tokeniser is a data-dependent switch per byte.
 *
 * THREE FORMS, because the element type is the thing most likely to decide it:
 *
 *   byte[]   what a person actually writes
 *   long[]   OUR memory shape -- our int is 64-bit (ADR 0012)
 *   String   charAt, which is what a JSON API would really be handed
 *
 * AND THIS IS THE FIRST PROGRAM TO MISS THE INDEX NARROWING.
 * indextype-2026-08-25 emits a counter as the host's own `int` when it is
 * bounded by a length and steps by +1, because a Java array index is 32-bit and
 * an emitted `long` counter costs 1.04x to 1.54x in casts. A tokeniser's `i`
 * steps by a SCANNER'S RETURN VALUE, so neither condition holds and the
 * narrowing does not fire: GenTokens carries 14 (int) casts. That result named
 * the sieve as not hitting this path; this is the path.
 *
 *   javac -d out gen/GenJsonTok.java JsonTokBench.java
 *   java -cp out JsonTokBench
 */
public final class JsonTokBench {

    static final int CAP = 32;
    static Object sink;

    // ---- hand-written references -------------------------------------------

    static long tokBytes(byte[] src) {
        byte[] stk = new byte[CAP];
        int i = 0, nt = 0, sp = 0, mx = 0, ok = 1;
        for (;;) {
            if (i >= src.length) return nt * 1000L + mx * 10L + (sp == 0 ? ok : 0);
            if (sp >= CAP) return nt * 1000L;
            int c = src[i] & 0xff;
            if (c == 32 || c == 9 || c == 10 || c == 13) { i++; continue; }
            if (c == 123 || c == 91) {
                stk[sp] = (byte) (c == 123 ? 125 : 93);
                i++; nt++; sp++;
                if (sp > mx) mx = sp;
                continue;
            }
            if (c == 125 || c == 93) {
                i++; nt++;
                if (sp < 1) { ok = 0; } else { sp--; if ((stk[sp] & 0xff) != c) ok = 0; }
                continue;
            }
            if (c == 58 || c == 44) { i++; nt++; continue; }
            if (c == 34) {
                int j = i + 1;
                for (;;) {
                    if (j >= src.length) break;
                    int d = src[j] & 0xff;
                    if (d == 92) { j += 2; continue; }
                    if (d == 34) { j++; break; }
                    j++;
                }
                i = j; nt++;
                continue;
            }
            if (isNum(c)) {
                int j = i;
                while (j < src.length && isNum(src[j] & 0xff)) j++;
                i = j; nt++;
                continue;
            }
            if (c >= 97 && c <= 122) {
                int j = i;
                while (j < src.length) { int d = src[j] & 0xff; if (d < 97 || d > 122) break; j++; }
                i = j; nt++;
                continue;
            }
            i++; ok = 0;
        }
    }

    static long tokLongs(long[] src) {
        long[] stk = new long[CAP];
        int i = 0, nt = 0, sp = 0, mx = 0, ok = 1;
        for (;;) {
            if (i >= src.length) return nt * 1000L + mx * 10L + (sp == 0 ? ok : 0);
            if (sp >= CAP) return nt * 1000L;
            long c = src[i];
            if (c == 32 || c == 9 || c == 10 || c == 13) { i++; continue; }
            if (c == 123 || c == 91) {
                stk[sp] = c == 123 ? 125 : 93;
                i++; nt++; sp++;
                if (sp > mx) mx = sp;
                continue;
            }
            if (c == 125 || c == 93) {
                i++; nt++;
                if (sp < 1) { ok = 0; } else { sp--; if (stk[sp] != c) ok = 0; }
                continue;
            }
            if (c == 58 || c == 44) { i++; nt++; continue; }
            if (c == 34) {
                int j = i + 1;
                for (;;) {
                    if (j >= src.length) break;
                    long d = src[j];
                    if (d == 92) { j += 2; continue; }
                    if (d == 34) { j++; break; }
                    j++;
                }
                i = j; nt++;
                continue;
            }
            if (isNum(c)) {
                int j = i;
                while (j < src.length && isNum(src[j])) j++;
                i = j; nt++;
                continue;
            }
            if (c >= 97 && c <= 122) {
                int j = i;
                while (j < src.length) { long d = src[j]; if (d < 97 || d > 122) break; j++; }
                i = j; nt++;
                continue;
            }
            i++; ok = 0;
        }
    }

    // OUR SHAPE NOW: the generated tokeniser takes short[], because its
    // signature says (array (int 0 255)) and this target declares that it holds
    // that range in a `short` -- the JVM's byte is SIGNED, so 0..255 does not
    // fit it, and nothing special-cases that. byte[] stays as the control for
    // what a person writes.

    static long tokShorts(short[] src) {
        short[] stk = new short[CAP];
        int i = 0, nt = 0, sp = 0, mx = 0, ok = 1;
        for (;;) {
            if (i >= src.length) return nt * 1000L + mx * 10L + (sp == 0 ? ok : 0);
            if (sp >= CAP) return nt * 1000L;
            long c = src[i];
            if (c == 32 || c == 9 || c == 10 || c == 13) { i++; continue; }
            if (c == 123 || c == 91) {
                stk[sp] = (short) (c == 123 ? 125 : 93);
                i++; nt++; sp++;
                if (sp > mx) mx = sp;
                continue;
            }
            if (c == 125 || c == 93) {
                i++; nt++;
                if (sp < 1) { ok = 0; } else { sp--; if (stk[sp] != c) ok = 0; }
                continue;
            }
            if (c == 58 || c == 44) { i++; nt++; continue; }
            if (c == 34) {
                int j = i + 1;
                for (;;) {
                    if (j >= src.length) break;
                    long d = src[j];
                    if (d == 92) { j += 2; continue; }
                    if (d == 34) { j++; break; }
                    j++;
                }
                i = j; nt++;
                continue;
            }
            if (isNum(c)) {
                int j = i;
                while (j < src.length && isNum(src[j])) j++;
                i = j; nt++;
                continue;
            }
            if (c >= 97 && c <= 122) {
                int j = i;
                while (j < src.length) { long d = src[j]; if (d < 97 || d > 122) break; j++; }
                i = j; nt++;
                continue;
            }
            i++; ok = 0;
        }
    }

    static int tokStringS(short[] a, int i) {
        int j = i + 1;
        for (;;) {
            if (j >= a.length) return j;
            if (a[j] == 92) { j += 2; continue; }
            if (a[j] == 34) return j + 1;
            j++;
        }
    }

    // THE SAME PROGRAM INDEXED BY A LONG, which is what the emitter produces
    // when index narrowing does not fire. If this measures like GEN then the
    // casts are the whole story; if it measures like tokLongs they are not.
    // indextype-2026-08-25 isolated Java's index cost this way and this is the
    // shape it said the sieve does not reach.
    static long tokLongsIdx(long[] src) {
        long[] stk = new long[CAP];
        long i = 0, nt = 0, sp = 0, mx = 0, ok = 1;
        for (;;) {
            if (i >= src.length) return nt * 1000L + mx * 10L + (sp == 0 ? ok : 0);
            if (sp >= CAP) return nt * 1000L;
            long c = src[(int) i];
            if (c == 32 || c == 9 || c == 10 || c == 13) { i++; continue; }
            if (c == 123 || c == 91) {
                stk[(int) sp] = c == 123 ? 125 : 93;
                i++; nt++; sp++;
                if (sp > mx) mx = sp;
                continue;
            }
            if (c == 125 || c == 93) {
                i++; nt++;
                if (sp < 1) { ok = 0; } else { sp--; if (stk[(int) sp] != c) ok = 0; }
                continue;
            }
            if (c == 58 || c == 44) { i++; nt++; continue; }
            if (c == 34) {
                long j = i + 1;
                for (;;) {
                    if (j >= src.length) break;
                    long d = src[(int) j];
                    if (d == 92) { j += 2; continue; }
                    if (d == 34) { j++; break; }
                    j++;
                }
                i = j; nt++;
                continue;
            }
            if (isNum(c)) {
                long j = i;
                while (j < src.length && isNum(src[(int) j])) j++;
                i = j; nt++;
                continue;
            }
            if (c >= 97 && c <= 122) {
                long j = i;
                while (j < src.length) { long d = src[(int) j]; if (d < 97 || d > 122) break; j++; }
                i = j; nt++;
                continue;
            }
            i++; ok = 0;
        }
    }

    static long tokString(String src) {
        char[] stk = new char[CAP];
        int i = 0, nt = 0, sp = 0, mx = 0, ok = 1;
        final int n = src.length();
        for (;;) {
            if (i >= n) return nt * 1000L + mx * 10L + (sp == 0 ? ok : 0);
            if (sp >= CAP) return nt * 1000L;
            char c = src.charAt(i);
            if (c == 32 || c == 9 || c == 10 || c == 13) { i++; continue; }
            if (c == 123 || c == 91) {
                stk[sp] = (char) (c == 123 ? 125 : 93);
                i++; nt++; sp++;
                if (sp > mx) mx = sp;
                continue;
            }
            if (c == 125 || c == 93) {
                i++; nt++;
                if (sp < 1) { ok = 0; } else { sp--; if (stk[sp] != c) ok = 0; }
                continue;
            }
            if (c == 58 || c == 44) { i++; nt++; continue; }
            if (c == 34) {
                int j = i + 1;
                for (;;) {
                    if (j >= n) break;
                    char d = src.charAt(j);
                    if (d == 92) { j += 2; continue; }
                    if (d == 34) { j++; break; }
                    j++;
                }
                i = j; nt++;
                continue;
            }
            if (isNum(c)) {
                int j = i;
                while (j < n && isNum(src.charAt(j))) j++;
                i = j; nt++;
                continue;
            }
            if (c >= 97 && c <= 122) {
                int j = i;
                while (j < n) { char d = src.charAt(j); if (d < 97 || d > 122) break; j++; }
                i = j; nt++;
                continue;
            }
            i++; ok = 0;
        }
    }

    static boolean isNum(long c) {
        return (c >= 48 && c <= 57) || c == 45 || c == 43 || c == 46 || c == 101 || c == 69;
    }

    // ---- input --------------------------------------------------------------

    static String makeDoc(int records) {
        StringBuilder b = new StringBuilder("{\"items\":[");
        for (int r = 0; r < records; r++) {
            if (r > 0) b.append(',');
            b.append("{\"id\":1234,\"name\":\"a b\\\"c\",\"tags\":[\"x\",\"y\",\"z\"],")
             .append("\"score\":-12.5e3,\"ok\":true,\"prev\":null,")
             .append("\"meta\":{\"depth\":2,\"flag\":false}}");
        }
        return b.append("]}").toString();
    }

    static byte[] bytesOf(String s) {
        byte[] out = new byte[s.length()];
        for (int i = 0; i < s.length(); i++) out[i] = (byte) s.charAt(i);
        return out;
    }

    static short[] shortsOf(String s) {
        short[] out = new short[s.length()];
        for (int i = 0; i < s.length(); i++) out[i] = (short) s.charAt(i);
        return out;
    }

    static long[] longsOf(String s) {
        long[] out = new long[s.length()];
        for (int i = 0; i < s.length(); i++) out[i] = s.charAt(i);
        return out;
    }

    // ---- harness ------------------------------------------------------------

    static void run(String what, java.util.function.Supplier<Object> f, int warm, int iters) {
        for (int i = 0; i < warm; i++) sink = f.get();
        double best = Double.MAX_VALUE;
        for (int r = 0; r < 9; r++) {
            long t0 = System.nanoTime();
            for (int i = 0; i < iters; i++) sink = f.get();
            double d = (System.nanoTime() - t0) / (double) iters;
            if (d < best) best = d;
        }
        System.out.printf("%-36s %10.1f ns%n", what, best);
    }

    public static void main(String[] args) {
        // Agreement first: a benchmark of four programs that disagree measures
        // nothing. The malformed inputs matter more than the well-formed ones.
        for (int n : new int[]{0, 1, 3, 17}) check(makeDoc(n));
        for (String s : new String[]{"]", "{\"a\":1", "[}", "{]", "", "[[[[1]]]]"}) check(s);
        System.out.println("all four agree");

        String doc = makeDoc(64);
        byte[] db = bytesOf(doc);
        long[] dl = longsOf(doc);
        short[] ds = shortsOf(doc);

        run("J  tokenize String     hand", () -> tokString(doc), 50000, 2000);
        run("J  tokenize byte[]     hand", () -> tokBytes(db), 50000, 2000);
        run("J  tokenize long[]     hand", () -> tokLongs(dl), 50000, 2000);
        run("J  tokenize long[]/long idx hand", () -> tokLongsIdx(dl), 50000, 2000);
        run("J  tokenize short[]    hand", () -> tokShorts(ds), 50000, 2000);
        run("J  tokenize short[]    GEN ", () -> GenJsonTok.GenTokens(ds), 50000, 2000);
    }

    static void check(String s) {
        long want = tokString(s);
        long[] l = longsOf(s);
        if (tokBytes(bytesOf(s)) != want || tokLongs(l) != want || tokLongsIdx(l) != want
                || tokShorts(shortsOf(s)) != want || GenJsonTok.GenTokens(shortsOf(s)) != want) {
            throw new AssertionError(s + ": " + want + " " + tokBytes(bytesOf(s))
                    + " " + tokLongs(l) + " " + tokShorts(shortsOf(s)));
        }
    }
}
