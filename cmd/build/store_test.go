package main

import (
	"strings"
	"testing"

	"oroboros/emit"
)

// A STORE IS DEFINED ON ITS TABLE'S DOMAIN (the book's finding 70). set(f, i,
// x) = f[i ↦ x] is a function on [0, len f) only for i in it, so a store owes
// 0 <= i < len as a read does (tables.md §6). Before, each refused program
// below compiled on Go, JavaScript and windows: Go panicked at run time and
// windows wrote past the region. In-bounds stores, a constant one and a guarded
// loop's, still build and print what they wrote.
func TestAStoreIsDefinedOnItsTablesDomain(t *testing.T) {
	const head = "use io\n\n"
	const ok = head +
		"fn last(n ∈ 1..10) {\n    var b := array { 0 : _ ∈ 0..n-1 }\n    b[n - 1] := 7\n    b[n - 1]\n}\n" +
		"fn fill(n ∈ 1..10) {\n    var b := array { 0 : _ ∈ 0..n-1 }\n    loop i = 0 {\n" +
		"        i < n -> { b[i] := i; again(i + 1) }\n        else -> b[n - 1]\n    }\n}\n" +
		"export fn main() {\n    io.print_int(last(5))\n    io.print_int(fill(5))\n}\n"
	refused := map[string]string{
		"one past the end": head +
			"fn f(n ∈ 1..10) {\n    var b := array { 0 : _ ∈ 0..n-1 }\n    b[n] := 1\n    b[0]\n}\n" +
			"export fn main() { io.print_int(f(5)) }\n",
		"an index in Int": head +
			"fn f(n ∈ 1..10, i ∈ Int) {\n    var b := array { 0 : _ ∈ 0..n-1 }\n    b[i] := 1\n    b[0]\n}\n" +
			"export fn main() { io.print_int(f(5, 3 - 4)) }\n",
		"the off-by-one loop": head +
			"fn f(n ∈ 1..10) {\n    var b := array { 0 : _ ∈ 0..n-1 }\n    loop i = 0 {\n" +
			"        i <= n -> { b[i] := i; again(i + 1) }\n        else -> b[0]\n    }\n}\n" +
			"export fn main() { io.print_int(f(5)) }\n",
	}
	for _, target := range []string{"go", "windows"} {
		out, err := buildAndRun(t, ok, target, "")
		if err != nil || strings.Join(strings.Fields(out), " ") != "7 4" {
			t.Errorf("%s: the in-bounds stores: %q, %v; want 7 and 4", target, out, err)
		}
		for what, src := range refused {
			_, err := buildAndRunNoExec(t, src, target)
			if err == nil || !strings.Contains(err.Error(), "is a store, defined only on its table's domain") {
				t.Errorf("%s: %s: want it refused as a store outside its table's domain, got %v", target, what, err)
			}
		}
	}

	// THE PLANTED FAULT, nothing owed at a store: the hole returns, and the
	// program one past the end builds again.
	emit.PlantStoreUnchecked = true
	defer func() { emit.PlantStoreUnchecked = false }()
	if _, err := buildAndRunNoExec(t, refused["one past the end"], "go"); err != nil {
		t.Errorf("with the store's domain unowed the program should build, so the test reaches the rule; got %v", err)
	}
}
