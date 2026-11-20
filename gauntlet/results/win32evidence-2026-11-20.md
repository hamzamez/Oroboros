# win32evidence — Win32's enumerators read against their pages, 2026-11-20

hamza, 2026-11-20: "Build all 5", item 5. ADR 0143 owed "ten of Win32's 291 enumerators read
against their pages before any is marked". The pages are Microsoft Learn's, read in the browser pane
with hamza's permission of 2026-11-20. Only the sentences about the count and the order are kept
here, each short and with its page.

## What each page promises

| function | the page's promise | count | order |
|---|---|---|---|
| EnumWindows | each top-level window "in turn", until the last or the callback returns FALSE | each once, an early stop | none stated |
| EnumChildWindows | each child window "in turn"; one moved in the Z order is still enumerated, one created or destroyed during it is not | each once, an early stop | none (the Z order explicitly may change) |
| EnumThreadWindows | each nonchild window "in turn" | each once, an early stop | none stated |
| EnumDisplayMonitors | the callback "once for each monitor" | each once | none stated |
| EnumSystemCodePages | identifiers "one at a time", until all are passed or FALSE | each once, an early stop | none stated |
| EnumSystemLocalesEx | names "one at a time", until all are passed or FALSE | each once, an early stop | none stated |
| EnumPropsExA | entries "one by one", until the last or FALSE | each once, an early stop | none stated |
| EnumSystemGeoID (declared) | no sentence on either | none stated | none stated |
| LineDDA | each point "along the line" but the last | each once | the points are along a line, the sequence unstated |
| InitOnceExecuteOnce (declared, `calls 0..1`) | "only one thread will execute the callback" | at most once | trivial |
| EnumFontFamiliesExA, EnumDateFormatsExEx | no sentence on either in the page's text | none stated | none stated |
| EnumResourceNamesA | the page was not found at the URL tried | — | — |

## What it says

- **No Win32 enumerator read promises an order**, so none is `ordered`, and the windows slot's
  restore keeps no witness: re-entry needs an effect in a λ whose API has no context word, and an
  effect needs an order (Theorem D). EnumSystemGeoID, the one slot API declared, promises neither.
- **They all promise the middle strength**: every element, once, one call after another ("in
  turn"), stopped early by the callback's FALSE, in no stated order. That is a bag traversed with
  an early exit, formers-research §5's level, and the most common promise Win32 makes. A cell
  written under it is sound exactly when its writes are a commutative monoid's: counting windows,
  summing monitors' areas, the maximum code page. The word for it is hamza's to decide (set aside
  2026-11-20), and this is the evidence it would rest on.
- **The early stop matters for the monoid.** "Until … FALSE" makes the traversal a prefix of an
  unordered sequence, and which prefix is the host's. So a cell's commutative writes are sound only
  when the callback never stops early, or its stopping is a function of the cell's own value (a
  search for one element, after which nothing more is written).

## Paid and bought

| | |
|---|---|
| compile time, run time, code size, proofs, refusals changed | none: no declaration changed |
| compiler and language lines | none |
| documents | this one |
| rule made less general | none |
| not built | an `ordered` Win32 host, since none of these is; the middle strength's word, for hamza |
