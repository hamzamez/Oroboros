# Nine programs that check the surveys are not paper

A survey reports what the target format can DECLARE. These nine check that a
declaration it generates can be BUILT AND RUN, which is the only thing that makes
a percentage a measurement. All of them need generated target files, so they live
here rather than in `examples/`.

Four are Win32's, three are Go's, one is the JVM's and one is JavaScript's.

```bash
go run gauntlet/stdlib/win32.go -emit /tmp/win32gen
mkdir -p /tmp/proj/windows && cp /tmp/win32gen/fileapi.oro /tmp/win32gen/errhandlingapi.oro /tmp/proj/windows/
cp gauntlet/stdlib/acceptance/wide-call.oro gauntlet/stdlib/acceptance/void-stmt.oro /tmp/proj/
```

```bash
go run ./cmd/build -checked -target=windows -targets "/tmp/proj;targets" -o /tmp/wide /tmp/proj/wide-call.oro
go run ./cmd/build -target=windows -targets "/tmp/proj;targets" -o /tmp/void /tmp/proj/void-stmt.oro
```

`wide-call` must print **2** — ERROR_FILE_NOT_FOUND — and `void-stmt` must print
**1**.

The generated modules go in a project-local layer, which is what
layers-2026-09-07 built: `Δ_T = L₁ ▷ L₂ ▷ …`, and the source's own directory is
the nearest layer, so 8,809 generated primitives can sit beside a program without
touching `targets/`.

## Go: `os-methods.oro`

`survey.go` reports what Go's standard library can declare. This checks that a
declaration it generates can be built and run — including a **method**, which is
3,098 of the 4,932 callable names and which the generator refused to write until
2026-09-09.

```bash
go run gauntlet/stdlib/survey.go -emit /tmp/gostd
mkdir -p /tmp/goproj/tg/go && cp /tmp/gostd/os.oro /tmp/goproj/tg/go/os-gen.oro
cp gauntlet/stdlib/acceptance/os-methods.oro /tmp/goproj/
go run ./cmd/build -target=go -targets "/tmp/goproj/tg;targets" -o /tmp/osm /tmp/goproj/os-methods.oro
/tmp/osm            # must print 64, which is `head -c 64 go.mod | wc -c`
```

(Corrected 2026-09-11: this recipe put the file at `/tmp/goproj/go/os.oro`, which
the library search path finds as module `go/os` and refuses — the trap described
under `io-reader` below. Nothing had run it since; `tooling_test.go` does now.)

**All nine are a test**: `go test ./gauntlet/stdlib/` runs every survey twice,
builds these programs from that run's declarations, and checks what the host
prints (tooling-2026-09-11). `-short` skips it; a missing toolchain skips its
host by name.

It emits exactly what a person would write:

```go
f, e1 := os.Open("go.mod")
b := make([]byte, 64)
n, e2 := f.Read(b)
_ = (f.Close())
```

`os.Open` is a constructor returning `(*File, error)` — declarable only since
multiresult-2026-09-06, and the reason `*os.File` is in the obtainable-type fixed
point at all. `(*File).Read` is a method, so the receiver is argument 0, which is
the convention Win32's `HANDLE` already used arriving on a host that has objects.

---

## Go: `io-reader.oro` — a concrete type where an interface is wanted

`io.ReadAll(r io.Reader)` called with an `*os.File`. **Go's own compiler inserts
the coercion**, so the emitted text is `io.ReadAll(f)` with no conversion syntax
at all; what the `(implements ptr-os-File io-Reader …)` edge buys is our type
checker not refusing a program the host accepts.

```bash
go run gauntlet/stdlib/survey.go -emit /tmp/gostd
mkdir -p /tmp/goproj/tg/go && cp /tmp/gostd/os.oro /tmp/goproj/tg/go/os-gen.oro
cp /tmp/gostd/io.oro /tmp/goproj/tg/go/io-gen.oro
cp gauntlet/stdlib/acceptance/io-reader.oro /tmp/goproj/
go run ./cmd/build -target=go -targets "/tmp/goproj/tg;targets" -o /tmp/ior /tmp/goproj/io-reader.oro
/tmp/ior            # must print `wc -c < go.mod`
```

The generated files go in `tg/go/` rather than `go/`, and under a name that is
not `os.oro`: the source's own directory is also the LIBRARY search path, so a
file at `<src>/go/os.oro` is found by `(use go/os)` as a module and read with the
wrong grammar. Layers make a target directory and a library directory the same
kind of thing, which is the cost of that.

**Every edge is host-verified.** The api manifest lists the EXPORTED API, so an
interface sealed by an unexported method — `ast.Decl` has `declNode()` — looks
satisfied by everything. 182 of 1,651 candidates were false and `go build` on the
generated `implements_check.go` named every one; `survey.go` drops them and emits
only the survivors. To see that check for yourself:

```bash
mkdir -p /tmp/ck && cp /tmp/gostd/implements_check.go /tmp/ck/
cd /tmp/ck && go mod init check && go build ./...   # must be silent
```

---

## Go: `struct-literal.oro` — a value the host never handed us

A struct is `Pi` over a finite set of LABELS, and on a host that HAS structs there
is no layout to invent, because the host builds it and we hold a token. So the
constructor is an ordinary primitive whose template is a composite literal, and
this checks one that NESTS: `image.Rectangle`'s two fields are `image.Point`.

```bash
go run gauntlet/stdlib/survey.go -emit /tmp/gostd
mkdir -p /tmp/goproj/tg/go && cp /tmp/gostd/image.oro /tmp/goproj/tg/go/image-gen.oro
cp gauntlet/stdlib/acceptance/struct-literal.oro /tmp/goproj/
go run ./cmd/build -target=go -targets "/tmp/goproj/tg;targets" -o /tmp/sl /tmp/goproj/struct-literal.oro
/tmp/sl             # must print 8 then 4
```

It emits

```go
r := ((image.Rectangle{Max: ((image.Point{X: 10, Y: 5})), Min: ((image.Point{X: 2, Y: 1}))}))
```

**Max before Min, because fields are sorted by name** — an unsorted map iteration
would make the emitter stop being a function of its input
(backend-2026-09-06), and that is why a generated parameter is named for its
FIELD rather than `a0`, `a1`.

**The form is decided by the method set.** `&T{...}` is a `*T` and `T{...}` is a
`T`; our checker compares type names and gets no auto-dereference, so the pointer
form is generated exactly when the manifest gives the type a pointer-receiver
method. `image.Rectangle` is value methods only and gets the value form;
`(&http.MaxBytesError{Limit: 5}).Error()` is the other path and prints
`http: request body too large`.

**And it prints two numbers rather than their product**, because
`(* (Rect.Dx r) (Rect.Dy r))` is refused: a host call's result has no declared
range and ADR 0019 is bounded by default. The refusal is correct — Go's `int` is
the host's word — and the program says only what it can prove.

---

## The JVM: `jvm-object.oro`

`gauntlet/stdlib/jvm.go` reports what the JDK can declare, from a manifest the
runtime writes about itself. This checks that the declarations build and run.

```bash
java gauntlet/stdlib/jdk/Dump.java > /tmp/jdk-api.txt
go run gauntlet/stdlib/jvm.go -api /tmp/jdk-api.txt -emit /tmp/jdkgen
mkdir -p /tmp/jproj/tg/java
cp /tmp/jdkgen/java-util.oro /tmp/jproj/tg/java/java-util-gen.oro
cp /tmp/jdkgen/java-lang.oro /tmp/jproj/tg/java/java-lang-gen.oro
cp gauntlet/stdlib/acceptance/jvm-object.oro /tmp/jproj/
go run ./cmd/build -target=java -targets "/tmp/jproj/tg;targets" -o /tmp/jvmobj /tmp/jproj/jvm-object.oro
java -cp /tmp/jvmobj Main      # must print `a, b, c` then 30
```

**30 is what `new java.util.Random(42).nextInt(100)` prints in Java** — a value
the JVM specifies, so the answer is checkable against the host rather than
against ourselves.

Four things it exercises, each a refusal that had to be answered. **`new`** is a
keyword rather than a function here, so 4,138 constructors exist where Go has a
handful of `NewT` — and the ones on an abstract class, which `getConstructors`
lists anyway, have to be skipped. **The coercion**: `StringJoiner` takes a
`CharSequence` and we hold a `string`, so `(implements string
java-lang-CharSequence …)` is what makes it legal, with no conversion syntax in
the emitted Java at all. **The overload**: `Random.nextInt` is two methods and
one name, so the second is `nextInt2` — overloading.md's `Println`/`Println2`
wart, which has 4,948 instances on this host (5,446 when first written, a figure the
committed tool does not print — tooling-2026-09-11). **The method chain**: `add` returns
the joiner, which is what makes a method a SOURCE in the obtainable fixed point
and not just a consumer — leaving that out understated Go by ten points until the
JVM survey was written.

## JavaScript: `js-shape.oro`

`gauntlet/stdlib/js.go` reports that the other two surveys' questions have no
content on this host: every type is `any`, so declarable is 100% by construction
and the obtainable fixed point has one type in its domain. What a JavaScript
declaration needs instead is a name, a call form and an ARITY.

```bash
node gauntlet/stdlib/jsdump.mjs > /tmp/js-api.txt
go run gauntlet/stdlib/js.go -api /tmp/js-api.txt -emit /tmp/jsgen
mkdir -p /tmp/sproj/tg/js
cp /tmp/jsgen/globalThis.oro /tmp/sproj/tg/js/global-gen.oro
cp /tmp/jsgen/node-path.oro /tmp/sproj/tg/js/path-gen.oro
cp gauntlet/stdlib/acceptance/js-shape.oro /tmp/sproj/
go run ./cmd/build -target=js -targets "/tmp/sproj/tg;targets" -o /tmp/jsshape.mjs /tmp/sproj/js-shape.oro
node /tmp/jsshape.mjs          # must print 7, then ABC, then a single dot
```

**The dot is the point.** `path.join.length` is 0, so the generated declaration
is `(prim join (none) any expr "path.join()")` — legal, buildable, runnable, and
unable to be passed a path. `Function.length` counts the parameters before the
first default or rest and a native function's are not introspectable at all, so
`Math.max.length` is 2 on a variadic function and `console.log.length` is 0. **The
host does not merely decline to give us types; it misreports the shape**, which
is the one thing a declaration needs from it.

It also found a compiler bug: a single-result JavaScript prim silently dropped
its `(import …)`, because only the several-results path collected one and every
prim that had ever carried an import on that host was fallible.

---

## What each Win32 one is for

**`wide-call.oro`** — `CreateFileA` takes SEVEN arguments, three past the four
Win64 passes in registers and one past the six the emitter used to reserve room
for. It also passes `(x64.null)`, which 681 entry points need and which did not
exist until 2026-09-08. Opening a file that is not there must set
ERROR_FILE_NOT_FOUND.

**`void-stmt.oro`** — `SetLastError` returns `void`, which the survey refused for
409 functions and the format could always say: a statement's value IS its first
argument. Round-tripped through `GetLastError`, so the answer is checked rather
than assumed.

**`signed-result.oro`** — `MulDiv(-7, 6, 2)` must print **1** and then **79**: the result is negative,
and −21 + 100 is 79. Until 2026-09-11 it printed **0** and **4294967375**, because every generated
result was read as `mov %r, rax` and a C `int` comes back in `EAX`, which zero-extends
(win32enum-2026-09-11). `wide-call.oro` called `MulDiv` with positive arguments and could not have
seen it. **A witness has to be able to fail.**

**`enum-param.oro`** — `GetFileInformationByHandleEx` takes an enum, which the survey refused as a
struct until 2026-09-11. It must print **0, 24, 87**: class `FileBasicInfo` fails on the buffer
length, class 9999 fails as an invalid parameter, so two enum values give two errors and the callee
read the one we passed. The first prediction, 6, was wrong and the program says so.

```bash
go run gauntlet/stdlib/win32.go -emit /tmp/win32gen
mkdir -p /tmp/proj/windows && cp /tmp/win32gen/WinBase.oro /tmp/proj/windows/
cp gauntlet/stdlib/acceptance/signed-result.oro gauntlet/stdlib/acceptance/enum-param.oro /tmp/proj/
go run ./cmd/build -checked -target=windows -targets "/tmp/proj;targets" -o /tmp/signed /tmp/proj/signed-result.oro
go run ./cmd/build -checked -target=windows -targets "/tmp/proj;targets" -o /tmp/enum /tmp/proj/enum-param.oro
```
