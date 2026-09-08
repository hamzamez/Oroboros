# Two programs that check the Win32 survey is not paper

`win32.go` reports what the target format can DECLARE. These two check that a
declaration it generates can be built and run, which is the only thing that makes
a percentage a measurement. Both need generated target files, so they live here
rather than in `examples/`, and the recipe is three commands.

```bash
go run gauntlet/stdlib/win32.go -emit /tmp/win32gen
mkdir -p /tmp/proj/windows && cp /tmp/win32gen/fileapi.oro /tmp/win32gen/errhandlingapi.oro /tmp/proj/windows/
cp gauntlet/stdlib/acceptance/*.oro /tmp/proj/
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

## What each one is for

**`wide-call.oro`** — `CreateFileA` takes SEVEN arguments, three past the four
Win64 passes in registers and one past the six the emitter used to reserve room
for. It also passes `(x64.null)`, which 681 entry points need and which did not
exist until 2026-09-08. Opening a file that is not there must set
ERROR_FILE_NOT_FOUND.

**`void-stmt.oro`** — `SetLastError` returns `void`, which the survey refused for
409 functions and the format could always say: a statement's value IS its first
argument. Round-tripped through `GetLastError`, so the answer is checked rather
than assumed.
