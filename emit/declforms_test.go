package emit

import (
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// THE DECLARATION FORMS OF theories.md §8, as a target file writes them. Each
// refusal is checked to fire and to name what was wrong; each legitimate shape
// beside it is checked to load, so a test cannot pass by refusing everything.

// mod is declarations inside the module m.
func mod(decls string) string { return "module m {\n" + decls + "\n}" }

func loadOne(t *testing.T, body string) (*Target, error) {
	t.Helper()
	dir := t.TempDir()
	writeTarget(t, filepath.Join(dir, "x"), "a", body)
	return LoadTargetLayers("x", []string{dir})
}

func TestASigSeparatesItsTwoKindsOfClaim(t *testing.T) {
	// Every clause in its place, and arity zero as (). (Today's notation's
	// misplaced clauses have no spelling in J's grammar: the header holds the
	// claims and the host body the host's.)
	tg, err := loadOne(t, `fn g(a ∈ Int, b ∈ Int) (r ∈ Int) pure
    where 0 <= a
    ensure 0 <= r {
    host expr "G(${a}, ${b})" import "g" checked g jump "l"
}
fn null() (ptr) pure { host expr "0" }`)
	if err != nil {
		t.Fatal(err)
	}
	g := tg.Prims["g"]
	if !g.Pure || g.Where == nil || g.Ensures == nil || g.Import != "g" || g.Checked != "g" ||
		g.Jump != "l" || g.Form != "G(%s, %s)" || len(g.Args) != 2 {
		t.Errorf("a clause was lost: %+v", g)
	}
	if n := tg.Prims["null"]; len(n.Args) != 0 || !n.Pure {
		t.Errorf("() must be arity zero: %+v", n)
	}
}

// A CONSTANT IS THE SIG IT MEANS: E(const) = sig, checked as structural equality
// of the loaded declarations, at the top level and inside a module.
func TestAConstIsTheSigItMeans(t *testing.T) {
	for _, wrap := range []func(string) string{
		func(d string) string { return d },
		mod,
	} {
		c, err := loadOne(t, wrap(`Max ∈ set { -7 } = host "p.Max" import "p"`))
		if err != nil {
			t.Fatal(err)
		}
		s, err := loadOne(t, wrap(`fn Max() (-7..-7) pure { host expr "p.Max" import "p" }`))
		if err != nil {
			t.Fatal(err)
		}
		for n, p := range s.Prims {
			if q := c.Prims[n]; !reflect.DeepEqual(p, q) {
				t.Errorf("%s: a const must load to the sig it elaborates to:\n const %+v\n sig   %+v", n, q, p)
			}
		}
		if len(c.Prims) != len(s.Prims) {
			t.Errorf("a const declared %d names where its sig declares %d", len(c.Prims), len(s.Prims))
		}
	}
	refused := map[string]string{
		`Pi ∈ set { 3.14 } = host "math.Pi"`: "not an integer literal",
		`S ∈ set { "x" } = host "p.S"`:       "not an integer literal",
		`N ∈ set { 4 } = host expr "p.N"`:    "has no kind",
	}
	for decl, why := range refused {
		if _, err := loadOne(t, decl); err == nil || !strings.Contains(err.Error(), why) {
			t.Errorf("%s: want a refusal containing %q, got %v", decl, why, err)
		}
	}
}

func TestReprAndFactAreRefusedOffTheirShapes(t *testing.T) {
	refused := map[string]string{
		`repr big medium`:                   "(repr big host) or (repr big limbs)",
		`repr 5..1 host "x"`:                "is empty",
		`repr shift 64`:                     "1 <= N <= 63",
		"repr `map` hash":                   "(repr map host) or (repr map library)",
		`repr float host "double"`:          "(repr (int LO HI)",
		`fact f (a ∈ Array(A)) len(a) < 3`:  "facts are specified",
		`fact f (a ∈ Array(A)) len(b) <= 3`: "facts are specified",
		`fact f (a ∈ Array(A)) len(a) <= 0`: "at least 1",
		`set t = "spelling"`:                "a host spelling goes inside",
	}
	for decl, why := range refused {
		if _, err := loadOne(t, decl); err == nil || !strings.Contains(err.Error(), why) {
			t.Errorf("%s: want a refusal containing %q, got %v", decl, why, err)
		}
	}
	tg, err := loadOne(t, `repr big limbs
repr shift 31
repr ref(Int) host "Long"
fact max_len (xs ∈ Array(A)) len(xs) <= (2^31-1)
set Map(K, V) = host "M<${K},${V}>"`)
	if err != nil {
		t.Fatal(err)
	}
	if tg.BigRepr != "limbs" || tg.ShiftWidth != 31 || tg.Boxed["int"] != "Long" ||
		tg.MaxLen != 2147483647 || tg.MapType != "M<%s,%s>" {
		t.Errorf("a declaration was lost: %+v", tg)
	}
}

// THE MAP REPRESENTATION COMPOSES BY OVERRIDE (theories.md §5.8). It was a flag
// joined with ||, so a nearer layer could turn the language's own map on and
// never off.
func TestANearerLayerCanTurnTheLibraryMapOff(t *testing.T) {
	root := t.TempDir()
	host, lib := filepath.Join(root, "host"), filepath.Join(root, "lib")
	writeTarget(t, filepath.Join(host, "x"), "a", "repr `map` host")
	writeTarget(t, filepath.Join(lib, "x"), "a", "repr `map` library")
	for _, c := range []struct {
		layers []string
		want   bool
	}{{[]string{host, lib}, false}, {[]string{lib, host}, true}} {
		tg, err := LoadTargetLayers("x", c.layers)
		if err != nil {
			t.Fatal(err)
		}
		if got := tg.NeedsMapImpl(); got != c.want {
			t.Errorf("layers %v: NeedsMapImpl is %v, want %v — the nearer layer must win", c.layers, got, c.want)
		}
	}
}

// A LIBRARY'S DECLARATION THAT DOES NOT LOAD IS REFUSED. loadProvides skipped
// every form but `prim` and ended a file's walk on the first error, in silence,
// so a native fast path could vanish from a build with no diagnostic.
func TestABrokenProvidesIsRefusedNotDropped(t *testing.T) {
	root := t.TempDir()
	targets, lib := filepath.Join(root, "targets"), filepath.Join(root, "lib")
	writeTarget(t, filepath.Join(targets, "x"), "a", `backend go`)
	writeTarget(t, filepath.Join(lib, "m"), "x", "provides x m {\n    fn f(a ∈ Int) (Int) pure\n}")
	_, err := LoadTargetLayers("x", []string{targets}, []string{lib})
	if err == nil || !strings.Contains(err.Error(), "x.oro") {
		t.Errorf("a provides with no realization must be refused naming its file, got %v", err)
	}
	writeTarget(t, filepath.Join(lib, "m"), "x", "provides x m {\n    fn f(a ∈ Int) (Int) pure { host expr \"F(${a})\" }\n}")
	tg, err := LoadTargetLayers("x", []string{targets}, []string{lib})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tg.Prims["m.f"]; !ok {
		t.Error("the corrected provides must contribute m.f")
	}
}

// AND ONE THAT DOES NOT PARSE IS REFUSED, NAMING THE FILE. loadProvides skipped
// a fragment the reader rejected — "the loader's problem, not ours" — but the
// module loader never reads a `provides`, so nobody reported it: the program was
// told `(use m)` matched no file. A bad escape in lib/os/java.oro made every Java
// program say `os` did not exist (hoststring-2026-09-24). A library file that
// says nothing about `provides` is still the module loader's to read.
func TestAProvidesThatDoesNotParseIsRefusedNamingTheFile(t *testing.T) {
	root := t.TempDir()
	targets, lib := filepath.Join(root, "targets"), filepath.Join(root, "lib")
	writeTarget(t, filepath.Join(targets, "x"), "a", `backend go`)
	writeTarget(t, filepath.Join(lib, "m"), "x", "provides x m {\n    fn f(a ∈ Int) (Int) pure { host expr \"F\\q(${a})\" }\n}")
	_, err := LoadTargetLayers("x", []string{targets}, []string{lib})
	if err == nil {
		t.Fatal("a provides fragment that does not parse was skipped in silence")
	}
	if !strings.Contains(err.Error(), filepath.Join(lib, "m", "x.oro")) || !strings.Contains(err.Error(), `\q is not an escape`) {
		t.Errorf("the refusal must name the file and the reader's message, got %v", err)
	}
	// A file with no `provides` in it is not this loader's to judge.
	writeTarget(t, filepath.Join(lib, "m"), "x", "fn g(x) { \"F\\q\" }")
	if _, err := LoadTargetLayers("x", []string{targets}, []string{lib}); err != nil {
		t.Errorf("a library file that declares no provides is the module loader's business: %v", err)
	}
}

// A TYPE IS A MEMBER OF ITS MODULE (theories.md §3.2, §3.4). A module in a
// target is a signature Σ = (S, Ω), and its sorts belong to it — so the key is
// the whole path, which is what makes resolution injective on types: a base name
// two packages share is two types, measured as 3 of Go's 1,270 exported type
// names (declaration-surface.md §4.2).
func TestATypeIsAMemberOfItsModule(t *testing.T) {
	tg, err := loadOne(t, "set Int = host \"int\"\nmodule go/io {\n    set Writer = host \"io.Writer\"\n}\nmodule go/text-template {\n    set Template = host \"template.Template\"\n}\nmodule go/html-template {\n    set Template = host \"template.Template\"\n}\nmodule go/encoding/hex {\n    fn NewEncoder(w ∈ `go/io`.Writer) (`go/io`.Writer) { host expr \"hex.NewEncoder(${w})\" }\n}")
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"go/io.Writer":              "io.Writer",
		"go/text-template.Template": "template.Template",
		"go/html-template.Template": "template.Template",
	} {
		if got := tg.Types[name]; got != want {
			t.Errorf("%s realizes %q, want %q", name, got, want)
		}
	}
	// THE COLLISION IS GONE: two distinct host types with one base name are two
	// declarations, where a flat pool keyed by the base name had one.
	if _, flat := tg.Types["Template"]; flat {
		t.Error("a module's type must not also land in the target's flat pool")
	}
	if p := tg.Prims["go/encoding/hex.NewEncoder"]; len(p.Args) != 1 || p.Args[0] != "go/io.Writer" {
		t.Errorf("a signature names another module's type by its path: %+v", p.Args)
	}
}

// A TYPE CONSTRUCTOR IS THE TARGET'S, not a module's: `array` and `map` are
// `lang`'s own and every typed target realizes each exactly once (§5.6).
func TestATypeConstructorIsNotAModuleMember(t *testing.T) {
	_, err := loadOne(t, `module go/io {
    set Array(A) = host "[]${A}"
}`)
	if err == nil || !strings.Contains(err.Error(), "outside any module") {
		t.Errorf("a constructor inside a module must be refused, got %v", err)
	}
}

// A TYPE'S IDENTITY IS ITS REALIZATION. Once a type is owned by its module, one
// host type can have two keys — a hand-written file's `go/io.Writer` and a
// generated survey's `io-Writer` — and a program using both must still pass a
// value from one to the other. The pool maps a name to the host's spelling, and
// for an opaque host type that spelling IS the type.
func TestOneHostTypeMayHaveTwoKeys(t *testing.T) {
	tg, err := loadOne(t, `set ptr_os_File = host "*os.File"
set io_Writer = host "io.Writer"
axiom ptr_os_File ⊆ io_Writer
module go/io {
    set Writer = host "io.Writer"
}`)
	if err != nil {
		t.Fatal(err)
	}
	if !tg.SameHostType("io-Writer", "go/io.Writer") {
		t.Error("two keys realizing io.Writer must name one type")
	}
	// AND THE EDGE IS READ THROUGH IT, on both ends: the subsumption was declared
	// with the generated key and is asked with the hand-written one.
	if !tg.Subsumes("ptr-os-File", "go/io.Writer") {
		t.Error("an implements edge must hold whichever key names its interface")
	}
	// The control: different spellings are different types, which is what stops
	// this from identifying everything.
	if tg.SameHostType("ptr-os-File", "go/io.Writer") {
		t.Error("*os.File and io.Writer are two host types")
	}
	if tg.Subsumes("go/io.Writer", "ptr-os-File") {
		t.Error("subsumption has a direction")
	}
}

// A COMPANION IS A TYPE'S METHOD SET, `include` IS THEORY INCLUSION, AND THE
// SUBTYPING EDGE IS DERIVED (theories.md §3.3, §6.1, §6.2).
//
// An interface had no way to carry a method, which is the wall encoding/hex
// recorded: a value of `io.WriteCloser` could be written to and never closed.
func TestACompanionIncludesAnotherAndTheEdgeFollows(t *testing.T) {
	tg, err := loadOne(t, "set error = host \"error\"\nset Int = host \"int\"\nmodule go/io {\n    set Writer = host \"io.Writer\"\n    set Closer = host \"io.Closer\"\n    set WriteCloser = host \"io.WriteCloser\"\n}\nmodule go/io/Writer {\n    fn Write(self ∈ `go/io`.Writer, p ∈ Int) (Int) { host expr \"${self}.Write(${p})\" }\n}\nmodule go/io/Closer {\n    fn Close(self ∈ `go/io`.Closer) (error) { host expr \"${self}.Close()\" }\n}\nmodule go/io/WriteCloser {\n    include `go/io/Writer` `go/io/Closer`\n}")
	if err != nil {
		t.Fatal(err)
	}
	// THE DECLARATIONS ARE THE INCLUDED ONES, with the receiver retyped: a method
	// of Writer applies to a WriteCloser because a WriteCloser is one.
	w, okW := tg.Prims["go/io/WriteCloser.Write"]
	_, okC := tg.Prims["go/io/WriteCloser.Close"]
	if !okW || !okC {
		t.Fatalf("include must bring both method sets: Write=%v Close=%v", okW, okC)
	}
	if w.Args[0] != "go/io.WriteCloser" || w.Args[1] != "int" || w.Form != "%s.Write(%s)" {
		t.Errorf("the receiver is retyped and nothing else moves: %+v", w)
	}
	// AND THE EDGE IS A THEOREM: methods(I) ⊆ methods(T) is what T ≤ I says.
	if !tg.Subsumes("go/io.WriteCloser", "go/io.Writer") || !tg.Subsumes("go/io.WriteCloser", "go/io.Closer") {
		t.Error("an inclusion must derive the subsumption edge")
	}
	if tg.Subsumes("go/io.Writer", "go/io.WriteCloser") {
		t.Error("subsumption has a direction: a Writer is not a WriteCloser")
	}
}

// AN EDGE IS A VIEW, AND A VIEW IS CHECKED (§6.1). It was declared and believed.
func TestAFalseImplementsEdgeIsRefused(t *testing.T) {
	decls := "set error = host \"error\"\n" +
		"module go/io {\n    set Writer = host \"io.Writer\"\n    set Closer = host \"io.Closer\"\n    axiom Writer ⊆ `go/io`.Closer\n}\n" +
		"module go/io/Closer {\n    fn Close(self ∈ `go/io`.Closer) (error) { host expr \"${self}.Close()\" }\n}\n"
	_, err := loadOne(t, decls)
	if err == nil || !strings.Contains(err.Error(), "is false") || !strings.Contains(err.Error(), "Close") {
		t.Errorf("an edge whose interface declares a method the subject does not must be refused, got %v", err)
	}
	// The control: state the method and the same edge is accepted.
	_, err = loadOne(t, decls+"module go/io/Writer {\n    fn Close(self ∈ `go/io`.Writer) (error) { host expr \"${self}.Close()\" }\n}\n")
	if err != nil {
		t.Errorf("the edge holds once the method is declared: %v", err)
	}
}

// A BUFFER AND A TABLE OF ONE ELEMENT ARE ONE HOST SLICE in a view (§6.1,
// gotarget-2026-09-30 §11): a hand interface declares Read as a write-borrow,
// and a generated implementation, which cannot know the host writes, takes an
// array. The element must still agree.
func TestAViewMeetsABufferWithATableOfItsElement(t *testing.T) {
	decls := func(implArg string) string {
		return "set error = host \"error\"\n" +
			"module go/io {\n    set Reader = host \"io.Reader\"\n    set R = host \"*R\"\n    axiom R ⊆ `go/io`.Reader\n}\n" +
			"module go/io/Reader {\n    fn Read(self ∈ `go/io`.Reader, p ∈ Array(0..255) linear) (buffer(0..255) × Int × error) {\n" +
			"        host expr \"${self}.Read(${p})\"\n    }\n}\n" +
			"module go/io/R {\n    fn Read(self ∈ `go/io`.R, p ∈ " + implArg + ") (Int × error) { host expr \"${self}.Read(${p})\" }\n}\n"
	}
	if _, err := loadOne(t, decls("Array(0..255)")); err != nil {
		t.Errorf("a generated array meets a hand buffer of the same element: %v", err)
	}
	if _, err := loadOne(t, decls("Array(Int)")); err == nil || !strings.Contains(err.Error(), "argument 2") {
		t.Errorf("an array of another element must not meet the buffer, got %v", err)
	}
}

func TestIncludeIsRefusedOffACompanion(t *testing.T) {
	for body, want := range map[string]string{
		"module go/io {\n    include `go/io/Writer`\n}":                                                             "not a companion",
		"set t = host \"T\"\nmodule go/io {\n    set A = host \"A\"\n}\nmodule go/io/A {\n    include `go/io/B`\n}": "no type go/io.B is declared",
		"module go/io {\n    set A = host \"A\"\n}\nmodule go/io/A {\n    include `go/io/A`\n}":                     "includes itself",
	} {
		if _, err := loadOne(t, body); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: want a refusal naming %q, got %v", body, want, err)
		}
	}
}

// A CONSTANT'S NAME IS A RANGE ENDPOINT, and it elaborates to the declaration
// the digits would have given — emit/constend.go, theories.md §8.3.
//
// The property is a commuting square: writing `(int 0 Max)` and writing
// `(int 0 1114111)` are the same declaration, so the Prim is DeepEqual either
// way. That is what makes it a spelling rather than a feature: nothing
// downstream can tell which was written.
func TestAConstantsNameIsARangeEndpoint(t *testing.T) {
	const c = `Max ∈ set { 1114111 } = host "u.Max" import "u"`
	named, err := loadOne(t, mod(c+"\n"+`fn f(r ∈ Int) (0..Max) pure { host expr "u.F(${r})" import "u" }`))
	if err != nil {
		t.Fatal(err)
	}
	digits, err := loadOne(t, mod(c+"\n"+`fn f(r ∈ Int) (0..1114111) pure { host expr "u.F(${r})" import "u" }`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(named.Prims["m.f"], digits.Prims["m.f"]) {
		t.Errorf("a named endpoint must be the declaration the digits give:\n named  %+v\n digits %+v",
			named.Prims["m.f"], digits.Prims["m.f"])
	}
	if got := named.Prims["m.f"].Result; got != "int 0 1114111" {
		t.Errorf("result %q, want int 0 1114111", got)
	}
	if len(named.Deferred) != 0 {
		t.Errorf("a successful load leaves nothing deferred, got %d", len(named.Deferred))
	}
}

// A CONSTANT MAY BE IN ANOTHER FILE, which is why resolution waits for the
// glue: a file is a fragment and load(F₁ ++ F₂) = load(F₁) ⊔ load(F₂), so
// splitting a file between a constant and its use must change nothing
// (loader-2026-09-15, TestSplittingAFileChangesNothing).
func TestAConstantEndpointResolvesAcrossFilesAndLayers(t *testing.T) {
	sig := `fn f(r ∈ Int) (0..Max) pure { host expr "u.F(${r})" import "u" }`
	cst := `Max ∈ set { 7 } = host "u.Max" import "u"`
	one, err := loadOne(t, mod(cst+"\n"+sig))
	if err != nil {
		t.Fatal(err)
	}
	// Two files in one layer, the USE read first — glue is order-free, and
	// sorting puts "a" before "b".
	dir := t.TempDir()
	writeTarget(t, filepath.Join(dir, "x"), "a", mod(sig))
	writeTarget(t, filepath.Join(dir, "x"), "b", mod(cst))
	split, err := LoadTargetLayers("x", []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(one.Prims["m.f"], split.Prims["m.f"]) {
		t.Errorf("splitting the file changed the declaration:\n one   %+v\n split %+v",
			one.Prims["m.f"], split.Prims["m.f"])
	}
	// And two LAYERS: the constant is in the far one, the use in the near one.
	near, far := t.TempDir(), t.TempDir()
	writeTarget(t, filepath.Join(near, "x"), "a", mod(sig))
	writeTarget(t, filepath.Join(far, "x"), "a", mod(cst))
	layered, err := LoadTargetLayers("x", []string{near, far})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(one.Prims["m.f"], layered.Prims["m.f"]) {
		t.Errorf("a constant in a farther layer was not found:\n one     %+v\n layered %+v",
			one.Prims["m.f"], layered.Prims["m.f"])
	}
}

// AN ENDPOINT THAT IS NOT A CONSTANT IS REFUSED, naming it. The singleton range
// IS the definiens, so a declaration that has no exact value is not one — and
// `+inf`/`-inf` stay endpoints of the grammar rather than becoming names to look
// up, which the control at the end checks.
func TestAnEndpointThatIsNotAConstantIsRefused(t *testing.T) {
	refused := map[string]string{
		`fn f(r ∈ Int) (0..Max) pure { host expr "F(${r})" }`: "is not declared in this target",
		`Max ∈ set { 7 } = host "u.Max"
fn g(r ∈ Int) (0..Max2) pure { host expr "G(${r})" }`: "Max2 is not declared",
		`fn Max(x ∈ Int) (7..7) pure { host expr "M(${x})" }
fn f(r ∈ Int) (0..Max) pure { host expr "F(${r})" }`: "it must be a CONSTANT",
		`fn Max() (0..7) pure { host expr "M()" }
fn f(r ∈ Int) (0..Max) pure { host expr "F(${r})" }`: "it must be a CONSTANT",
		`fn Max() (7..7) { host expr "M()" }
fn f(r ∈ Int) (0..Max) pure { host expr "F(${r})" }`: "not pure",
	}
	for decl, why := range refused {
		if _, err := loadOne(t, mod(decl)); err == nil ||
			!strings.Contains(err.Error(), why) {
			t.Errorf("%s: want a refusal containing %q, got %v", decl, why, err)
		}
	}
	// THE CONTROL: an infinite endpoint is not a constant name.
	tg, err := loadOne(t, "module m {\n    fn f(r ∈ Int) (0..`+inf`) pure { host expr \"F(${r})\" }\n}")
	if err != nil {
		t.Fatal(err)
	}
	if got := tg.Prims["m.f"].Result; got != "int 0 +inf" {
		t.Errorf("result %q, want int 0 +inf", got)
	}
}

// A MANIFEST TYPE IS WHAT IT MEANS — `(type NAME τ)`, a type with a DEFINIENS
// and no realization (theories.md §2's four cells at the type level).
//
// The property is the same commuting square a constant endpoint gets, one level
// up: unfolding is δ, so a declaration written with the name is the declaration
// written with what it stands for, and no backend, analysis or generator learns
// that aliases exist.
func TestAManifestTypeIsWhatItMeans(t *testing.T) {
	sig := func(ty string) string {
		return "fn f(x ∈ " + ty + ", p ∈ Array(" + ty + ")) (" + ty + ") pure { host expr \"F(${x},${p})\" }"
	}
	named, err := loadOne(t, "set u8 = 0..255\n"+mod(sig("u8")))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := loadOne(t, mod(sig("0..255")))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(named.Prims["m.f"], plain.Prims["m.f"]) {
		t.Errorf("a manifest type must be the declaration it means:\n named %+v\n plain %+v",
			named.Prims["m.f"], plain.Prims["m.f"])
	}
	// AND IT IS UNFOLDED TO A FIXPOINT, because an alias may be written in terms
	// of another — and the definiens may be in another file or another layer,
	// which is why the unfolding waits for the glue.
	near, far := t.TempDir(), t.TempDir()
	writeTarget(t, filepath.Join(near, "x"), "a", "set b = Array(u8)\n"+mod(sig("b")))
	writeTarget(t, filepath.Join(far, "x"), "a", `set u8 = 0..255`)
	chain, err := LoadTargetLayers("x", []string{near, far})
	if err != nil {
		t.Fatal(err)
	}
	if got := chain.Prims["m.f"].Result; got != "array int 0 255" {
		t.Errorf("result %q, want array int 0 255", got)
	}
}

// AND WHAT IT MAY NOT BE. Each refusal is a rule with a reason: a cycle has no
// normal form, a language type is not a target's to define, several results are
// read from the declaration itself, and a host spelling belongs in `(host …)` —
// which is what makes "no host clause" mean "no claim about any host".
func TestAManifestTypeIsRefusedOffItsRules(t *testing.T) {
	refused := map[string]string{
		`set a = Array(b)
set b = Array(a)`: "defined in terms of itself",
		`set a = Array(a)`:   "defined in terms of itself",
		`set Int = 0..255`:   "the language's own type",
		`set Array = 0..255`: "the language's own type",
		`set p = Int × Int`:  "may not stand for a tuple",
		`set t = "spelling"`: `goes inside (host …)`,
		`set t = host "T"
set t = 0..1`: "a type has one definition",
	}
	for decl, why := range refused {
		if _, err := loadOne(t, decl); err == nil ||
			!strings.Contains(err.Error(), why) {
			t.Errorf("%s: want a refusal containing %q, got %v", decl, why, err)
		}
	}
}

// `D_T` — A TARGET MAY DEFINE, NOT ONLY REALIZE. target-system.md §6.2,
// theories.md §5.4.
//
// Implementation selection is `P_T ▷ D_T ▷ D`, the same `▷` used everywhere
// else, and nothing in the reducer changes: a name in `D_T` is more entries in
// the definition environment and δ unfolds it exactly as it unfolds a library's.
func TestATargetMayDefineAndNotOnlyRealize(t *testing.T) {
	tg, err := loadOne(t, `module m {
    use n as q
    fn f(a) { q.g(a) }
}`)
	if err != nil {
		t.Fatal(err)
	}
	got := tg.Defs["m"]
	if len(got) != 2 || got[0].Kind != "use" || got[0].Alias != "q" || got[0].Name != "n" ||
		got[1].Kind != "def" || got[1].Name != "f" {
		t.Fatalf("D_T for module m is %+v", got)
	}
	if _, isPrim := tg.Prims["m.f"]; isPrim {
		t.Error("a definition is not a primitive: reduction must unfold it, not halt on it")
	}
	refused := map[string]string{
		`fn f(a) { a }`:     "belongs to a module",
		`use n`:             "belongs to a module",
		mod("f = 1\nf = 2"): "m.f is defined twice",
	}
	for decl, why := range refused {
		if _, err := loadOne(t, decl); err == nil || !strings.Contains(err.Error(), why) {
			t.Errorf("%s: want a refusal containing %q, got %v", decl, why, err)
		}
	}
}

// AND IT COMPOSES PER NAME LIKE EVERYTHING ELSE: a repeat within a layer is a
// mistake, and between layers the nearer one wins.
func TestATargetsDefinitionsGlueAndOverride(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	writeTarget(t, filepath.Join(a, "x"), "a", mod("f = 1"))
	writeTarget(t, filepath.Join(b, "x"), "a", mod("f = 2\ng = 3"))
	near, err := LoadTargetLayers("x", []string{a, b})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range near.Defs["m"] {
		names = append(names, f.Name+"="+f.Term.String())
	}
	if !reflect.DeepEqual(names, []string{"f=1", "g=3"}) {
		t.Errorf("the nearer layer's f must win and g must still arrive; got %v", names)
	}
	// Within ONE layer the same two files are a collision.
	one := t.TempDir()
	writeTarget(t, filepath.Join(one, "x"), "a", mod("f = 1"))
	writeTarget(t, filepath.Join(one, "x"), "b", mod("f = 2"))
	if _, err := LoadTargetLayers("x", []string{one}); err == nil ||
		!strings.Contains(err.Error(), "m.f is defined twice") {
		t.Errorf("want a glue collision naming m.f, got %v", err)
	}
}

// A MODULE MAY CONTAIN A MODULE (target-files.md §1a, ADR 0025's second half).
//
// A module path is a word in Seg* and a set of paths ordered by prefix IS a
// trie, so nesting adds no structure: it writes the trie as a tree, and the
// loader erases it by concatenation. The test is therefore a COMMUTING SQUARE —
// the nested spelling and the flat one must load to the SAME target, not to
// targets that behave alike.
func TestANestedModuleIsItsParentsPathAndItsOwn(t *testing.T) {
	nested := "set Int = host \"int\"\nmodule go/io {\n    set Writer = host \"io.Writer\"\n}\nmodule go/encoding/hex {\n    fn EncodeToString(src ∈ Array(Int)) (String) pure { host expr \"hex.EncodeToString(${src})\" }\n    module InvalidByteError {\n        fn Error(self ∈ `go/encoding/hex`.InvalidByteError) (String) pure {\n            host expr \"${self}.Error()\"\n        }\n    }\n    module Dumper {\n        fn Close(self ∈ `go/io`.Writer) (Int) { host expr \"${self}.Close()\" }\n    }\n}"
	flat := "set Int = host \"int\"\nmodule go/io {\n    set Writer = host \"io.Writer\"\n}\nmodule go/encoding/hex {\n    fn EncodeToString(src ∈ Array(Int)) (String) pure { host expr \"hex.EncodeToString(${src})\" }\n}\nmodule go/encoding/hex/InvalidByteError {\n    fn Error(self ∈ `go/encoding/hex`.InvalidByteError) (String) pure {\n        host expr \"${self}.Error()\"\n    }\n}\nmodule go/encoding/hex/Dumper {\n    fn Close(self ∈ `go/io`.Writer) (Int) { host expr \"${self}.Close()\" }\n}"
	a, err := loadOne(t, nested)
	if err != nil {
		t.Fatalf("nested: %v", err)
	}
	b, err := loadOne(t, flat)
	if err != nil {
		t.Fatalf("flat: %v", err)
	}
	if !reflect.DeepEqual(a.Prims, b.Prims) {
		t.Errorf("the two spellings must load to one target:\nnested %v\nflat   %v",
			names(a.Prims), names(b.Prims))
	}
	if _, ok := a.Prims["go/encoding/hex/InvalidByteError.Error"]; !ok {
		t.Errorf("a child's path is its parent's path and its own, got %v", names(a.Prims))
	}
}

// DEPTH IS UNBOUNDED, because concatenation is associative: nesting three deep
// is the same word as writing the three segments out.
func TestNestingIsAssociative(t *testing.T) {
	deep, err := loadOne(t, `set Int = host "int"
module go {
    module encoding {
        module hex {
            fn DecodedLen(n ∈ Int) (Int) pure { host expr "hex.DecodedLen(${n})" }
        }
    }
}`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := deep.Prims["go/encoding/hex.DecodedLen"]; !ok {
		t.Errorf("three nested modules are one path, got %v", names(deep.Prims))
	}
	// and a child may itself carry a path, since `/` is the monoid's operation
	wide, err := loadOne(t, `set Int = host "int"
module go {
    module encoding/hex {
        fn DecodedLen(n ∈ Int) (Int) pure { host expr "hex.DecodedLen(${n})" }
    }
}`)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(deep.Prims, wide.Prims) {
		t.Errorf("(a (b c)) and (a b/c) are one path:\n%v\n%v", names(deep.Prims), names(wide.Prims))
	}
}

// AN EMPTY SEGMENT IS REFUSED, which is the one thing concatenation must not
// silently do: `a` ++ "/" ++ "/b" would be a path no `use` can name. In J a
// module's path is segments joined by `/`, so the grammar refuses it, and the
// loader's own refusal ("empty segment") remains for a raw form.
func TestANestedModuleRefusesAnEmptySegment(t *testing.T) {
	for _, child := range []string{"/InvalidByteError", "InvalidByteError/"} {
		_, err := loadOne(t, "set Int = host \"int\"\nmodule go/encoding/hex {\n    module "+child+" {\n"+
			"        fn Error(n ∈ Int) (Int) pure { host expr \"${n}\" }\n    }\n}")
		if err == nil {
			t.Errorf("%q: an empty segment must be refused", child)
		}
	}
}

// names is a target's primitive names, sorted, for a readable failure.
func names(prims map[string]Prim) []string {
	var out []string
	for k := range prims {
		if strings.Contains(k, "/") {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
