// Conformance runner for the Java target. See README.md.
//
//   javac -encoding UTF-8 -d . Conform.java && java -cp . Conform
//
// The -encoding flag is not optional: javac defaults to the platform charset,
// which silently reinterprets the UTF-8 cases. See docs/spec/strings.md §5.
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.regex.MatchResult;
import java.util.regex.Pattern;

public class Conform {
    public static void main(String[] args) throws Exception {
        String src = new String(Files.readAllBytes(Path.of("cases.json")), StandardCharsets.UTF_8);
        List<String> cases = parseJsonStringArray(src);

        // The lowering declared in targets/java.oro.
        Pattern p = Pattern.compile("[^\\p{IsWhite_Space}]+");

        StringBuilder out = new StringBuilder();
        for (String s : cases) {
            String[] f = p.matcher(s).results().map(MatchResult::group).toArray(String[]::new);
            out.append(f.length).append(' ').append(jsonArray(f)).append('\n');
        }
        System.out.write(out.toString().getBytes(StandardCharsets.UTF_8));
        System.out.flush();
    }

    // A JSON string-array reader, so the three runners share one input file
    // without any of them depending on a library the others do not have.
    static List<String> parseJsonStringArray(String s) {
        List<String> out = new ArrayList<>();
        int i = s.indexOf('[');
        while (i < s.length()) {
            int q = s.indexOf('"', i + 1);
            if (q < 0) break;
            StringBuilder b = new StringBuilder();
            int j = q + 1;
            while (s.charAt(j) != '"') {
                char c = s.charAt(j);
                if (c == '\\') {
                    char e = s.charAt(++j);
                    switch (e) {
                        case 'n': b.append('\n'); break;
                        case 't': b.append('\t'); break;
                        case 'r': b.append('\r'); break;
                        case 'u':
                            b.append((char) Integer.parseInt(s.substring(j + 1, j + 5), 16));
                            j += 4;
                            break;
                        default: b.append(e);
                    }
                } else {
                    b.append(c);
                }
                j++;
            }
            out.add(b.toString());
            i = j + 1;
        }
        return out;
    }

    static String jsonArray(String[] f) {
        StringBuilder b = new StringBuilder("[");
        for (int i = 0; i < f.length; i++) {
            if (i > 0) b.append(',');
            b.append('"');
            for (char c : f[i].toCharArray()) {
                if (c == '"' || c == '\\') b.append('\\').append(c);
                else if (c == '\n') b.append("\\n");
                else if (c == '\t') b.append("\\t");
                else b.append(c);
            }
            b.append('"');
        }
        return b.append(']').toString();
    }
}
