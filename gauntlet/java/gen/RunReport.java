// Behavioural check for gauntlet program 5 on Java.
//
//   javac -encoding UTF-8 -d out gen/GenReport.java gen/RunReport.java
//   java -cp out RunReport
//
// The -encoding flag is not optional. See docs/spec/strings.md §5.
public final class RunReport {
    public static void main(String[] args) {
        double[] xs = {1.0, 0.0, 0.0, 0.0};
        GenReport.genReport("totals", xs);
    }
}
