// Hand-written references for examples/hash on the JVM (hashbench-2026-10-27).
//   java Hash.java MODE FILE    MODE: fnv | crc32
import java.nio.file.*;

public class Hash {
    public static void main(String[] a) throws Exception {
        byte[] b = Files.readAllBytes(Path.of(a[1]));
        if (a[0].equals("fnv")) {
            int h32 = 0x811C9DC5;
            for (byte c : b) { h32 ^= c & 0xFF; h32 *= 16777619; }
            long h64 = 0xCBF29CE484222325L;
            for (byte c : b) { h64 ^= c & 0xFF; h64 *= 1099511628211L; }
            System.out.printf("%08x%n%016x%n", h32, h64);
        } else {
            int[] tab = new int[256];
            for (int i = 0; i < 256; i++) {
                int c = i;
                for (int j = 0; j < 8; j++) c = (c & 1) == 1 ? 0xEDB88320 ^ (c >>> 1) : c >>> 1;
                tab[i] = c;
            }
            int crc = -1;
            for (byte v : b) crc = tab[(crc ^ v) & 0xFF] ^ (crc >>> 8);
            System.out.printf("%08x%n", ~crc);
        }
    }
}
