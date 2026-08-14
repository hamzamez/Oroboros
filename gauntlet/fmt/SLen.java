public class SLen {
    public static void main(String[] a) {
        String[] xs = { "abc", "café", "日本", "🙂", "e\u0301" };
        for (String s : xs) System.out.println(s.length());
    }
}
