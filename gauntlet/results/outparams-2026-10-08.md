# Out-parameters on Go, Win32, the JVM and the browser, measured

2026-10-08. The measurements behind [outparams-research.md](../../docs/outparams-research.md).

## 1. Go: explicit pointers to values are not out-parameters

Method: every `func` and `method` line of `$GOROOT/api/go1*.txt` (11,928 distinct), filtered for a
parameter whose type is a pointer to a value type (`*int…`, `*uint…`, `*string`, `*bool`, `*float…`,
`*complex…`, `*byte`, `*rune`, `*uintptr`), `syscall`, `runtime` and `unsafe` excluded.

**49**, in two packages:

| package | count | what the pointer is |
|---|---|---|
| `sync/atomic` | 35 | a cell shared between goroutines, read and written in place |
| `flag` | 14 | `IntVar(p *int, …)`: the address is kept and written at `flag.Parse` |

Neither is a result: both make the cell aliased (outparams-research §1.2).

## 2. Go: the out-parameters are typed `any`

Method: `go run gauntlet/stdlib/outparams.go` parses every non-test, non-internal file of Go's source,
and lists exported functions and methods with an `any` parameter whose doc comment says the argument
is stored through, must be a pointer, or is decoded into. 30 hits, read one at a time. Set aside:
encoders and sizes that read their argument (`binary.Write`, `binary.Size`, `gob.Encoder.Encode`,
`json/v2.Marshal`), `runtime.Pinner.Pin`, `runtime.SetFinalizer`, the experimental `json/v2` and an
internal generator. `fmt.Scanln`, `Sscanln` and `Fscanln` say only "similar to Scan" and are added by
reading.

| package | caller-typed out-parameters |
|---|---|
| `fmt` | `Scan`, `Scanf`, `Scanln`, `Sscan`, `Sscanf`, `Sscanln`, `Fscan`, `Fscanf`, `Fscanln` |
| `encoding/json` | `Unmarshal`, `Decoder.Decode` |
| `encoding/xml` | `Unmarshal`, `Decoder.Decode`, `Decoder.DecodeElement` |
| `encoding/gob` | `Decoder.Decode` |
| `encoding/asn1` | `Unmarshal` |
| `encoding/binary` | `Read`, `Decode` |
| `database/sql` | `Row.Scan`, `Rows.Scan` |
| `errors` | `As` |

About 25 across 8 packages. In each, the type the host writes is chosen by the caller's pointer, which
Go reads by reflection.

## 3. Go: what a cell costs

Method: `go build -gcflags=-m` on hand-written `var a int; var b string; fmt.Sscan(s, &a, &b)` and
`var pe *fs.PathError; errors.As(err, &pe)`.

```
moved to heap: a
moved to heap: b
moved to heap: pe
```

The address is boxed into `any`, so Go allocates each cell. The language's realization is this code,
so it is at parity by construction.

## 4. Win32: out-cells everywhere, each typed by its declaration

Method: a temporary probe on `gauntlet/stdlib/win32.go`'s parse of the SDK (10.0.26100.0; 1,753 headers,
11,575 flat entry points), counting per function the parameters whose SAL is an out-cell (`_Out_`,
`_Out_opt_`, `_Outptr_…`, `_Out_range_…`) and an out-buffer (`_Out_writes…`, `_Out_bytes…`, `_Out_z_`,
`_Out_cap…`). The probe was not committed. Folding it into `win32.go` as a report section is the
first step of building outparams-research's decision 1.

| | functions |
|---|---|
| with an out-cell only | 2,907 |
| with an out-buffer only | 977 |
| with both | 554 |
| **with at least one out-cell** | **3,461 (29.9%)** |

There are 4,598 out-cell parameters. By the SDK's naming (`P`/`LP` + a word type such as `DWORD`,
`ULONG`, `HANDLE`, `BOOL`…), **at least 2,133 point to one machine word**. The name list misses
`UINT32*` (96), `int*` (22), `bool*` (14), `PULONGLONG` (17) and others, so it is a lower bound. The
most frequent remaining pointees are pointers to pointers, an object the host allocates and hands back,
one word too: `EAP_ERROR**` 52, `BYTE**` 41, `void**` 41, `PSID*` 41, `PACL*` 32, `ID3DBlob**` 19.
Structs: `GUID*` 20, `LPRECT` 20, `LPFILETIME` 19, `LPSIZE` 17, `LPPOINT` 15. The survey's existing
report also counts 3,756 SAL "direction: out" annotations, 11.0% of all annotations.

Every out-cell's type is the declaration's. Win32 has no caller-typed out-parameter.

## 5. The JVM and the browser

Not measured by survey; read from the platforms' documentation.
- **Android**: out-parameters are caller-allocated arrays and objects, and are named so:
  `View.getLocationOnScreen(int[] outLocation)`, `View.getHitRect(Rect outRect)`,
  `Location.distanceBetween(…, float[] results)`, `Paint.getTextBounds(String, int, int, Rect bounds)`.
  An array is a write-borrow (host-buffers.md), and an object is a handle a method changes.
- **The browser**: Web APIs return results (`TextEncoder.encodeInto` returns `{read, written}`) and
  fill typed arrays (`crypto.getRandomValues`). Neither needs a value cell.

## 6. Not measured

- The JVM's methods taking a single-element holder array: no class-file survey reads parameter names.
- How many Win32 cells sit beside an `_Inout_` length (in-out, outparams-research §5.6).
