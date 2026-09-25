package java

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// print runs gen's front half on a source and prints every export through the
// IR, as `gen -printer java` does, into one class.
func print(t *testing.T, src, class string) string {
	t.Helper()
	src = filepath.Join("..", "..", src)
	layers, err := emit.SearchPath(src, filepath.Join("..", "..", "targets"))
	if err != nil {
		t.Fatal(err)
	}
	dirs := []string{filepath.Dir(src), filepath.Join("..", "..", "lib")}
	tg, err := emit.LoadTargetLayers("java", layers, dirs)
	if err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(string(text))
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.LoadWithDefs(forms, func(path string) (string, bool, error) {
		for _, d := range dirs {
			if b, err := os.ReadFile(filepath.Join(d, filepath.FromSlash(path)+".oro")); err == nil {
				return string(b), true, nil
			}
		}
		return "", false, nil
	}, tg.Defs)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	reqs := emit.InstallRequires(env, prog)
	exports := append([]string(nil), prog.Exports...)
	sort.Strings(exports)
	funcs := map[string]string{}
	for _, q := range exports {
		name := q[strings.LastIndex(q, ".")+1:]
		nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		if nf, err = emit.DischargeRequires(reqs, tg, name, prog.Sigs[q], nf); err != nil {
			t.Fatal(err)
		}
		code, err := FromResidual(tg, name, prog.Sigs[q], nf)
		if err != nil {
			t.Fatal(err)
		}
		funcs[name] = code
	}
	return emit.JavaFile(class, funcs)
}

func has(t *testing.T, code, want, why string) {
	t.Helper()
	if !strings.Contains(code, want) {
		t.Errorf("missing %q (%s) in\n%s", want, why, code)
	}
}

// method is one method's text.
func method(t *testing.T, code, name string) string {
	t.Helper()
	i := strings.Index(code, " "+name+"(")
	if i < 0 {
		t.Fatalf("%s not found in\n%s", name, code)
	}
	m := code[i:]
	if k := strings.Index(m, "\n\tpublic static"); k >= 0 {
		m = m[:k]
	}
	return m
}

// TestTheMeasuredShapes: the Java decisions of spec §9.4.
func TestTheMeasuredShapes(t *testing.T) {
	dot := print(t, "examples/native/dot-java.oro", "Dot")
	has(t, dot, "for (;; ", "PostVars: the index steps in the post clause")
	if strings.Contains(dot, "(int)") {
		t.Errorf("dot's index is not Java's int (a cast at every access):\n%s", dot)
	}
	code := print(t, "ir/testdata/java-shapes.oro", "Shapes")
	sum := method(t, code, "last")
	if strings.Contains(sum, "(int)") {
		t.Errorf("an index proven below 2³¹ takes a cast:\n%s", sum)
	}
	sq := method(t, code, "square")
	has(t, sq, "int v", "n lies in Java's int, and a parameter is held as its range says")
	has(t, sq, "(long) ", "n·n leaves int, so it is computed in long")
}

// TestTheScalarRepresentationRuns: the swap's parallel move and the product
// computed in its result's representation, under the JVM when it is on the
// path. A printer that multiplied in `int` gives 1410065408 for the square.
func TestTheScalarRepresentationRuns(t *testing.T) {
	code := print(t, "ir/testdata/java-shapes.oro", "Shapes")
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("javac is not on the path")
	}
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java is not on the path")
	}
	dir := t.TempDir()
	main := `public final class Main {
	public static void main(String[] args) {
		StringBuilder b = new StringBuilder();
		for (int n = 0; n < 4; n++) b.append(Shapes.swapSum(n)).append(' ');
		b.append(Shapes.square(100000)).append(' ');
		b.append(Shapes.last(new short[]{5, 6, 7}, 3));
		System.out.println(b);
	}
}
`
	for name, text := range map[string]string{"Shapes.java": code, "Main.java": main} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if out, err := exec.Command(javac, "-d", dir, filepath.Join(dir, "Shapes.java"), filepath.Join(dir, "Main.java")).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s\n%s", err, out, code)
	}
	out, err := exec.Command(java, "-cp", dir, "Main").CombinedOutput()
	if err != nil {
		t.Fatalf("java: %v\n%s", err, out)
	}
	// a, b start 1, 2 and swap n times; the result is a + 10b.
	if got, want := strings.TrimSpace(string(out)), "21 12 21 12 10000000000 7"; got != want {
		t.Fatalf("got %q, want %q\n%s", got, want, code)
	}
}
