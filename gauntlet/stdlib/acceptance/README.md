# Four programs that check the surveys are not paper

A survey reports what the target format can DECLARE. These four check that a
declaration it generates can be BUILT AND RUN, which is the only thing that makes
a percentage a measurement. All of them need generated target files, so they live
here rather than in `examples/`.

Two are Win32's and two are Go's; the Go ones are below.

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
mkdir -p /tmp/goproj/go && cp /tmp/gostd/os.oro /tmp/goproj/go/
cp gauntlet/stdlib/acceptance/os-methods.oro /tmp/goproj/
go run ./cmd/build -target=go -targets "/tmp/goproj;targets" -o /tmp/osm /tmp/goproj/os-methods.oro
/tmp/osm            # must print 64, which is `head -c 64 go.mod | wc -c`
```

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
