# Run every case of native.mjs in its OWN node process, pinned to one core.
#
# Two separate hazards, and both were caught by getting contradictory numbers
# rather than by reasoning about them:
#
#   1. This machine is bimodal on a hybrid P/E-core laptop. Unpinned runs of
#      the same benchmark differ by 3-6x (native-gauntlet-2026-08-20 section 9).
#      -Pin is OFF by default here, unlike the Go side, and that was measured
#      rather than assumed: pinning node to core 0 gave a WIDER spread than
#      leaving it alone (the allocating stencil read 282k/287k/288k pinned and
#      236k/253k/233k unpinned, against a hand-written 249k). Node runs its own
#      threads and pinning them all to one core adds contention that Go's
#      single-threaded benchmark does not have.
#   2. V8 carries optimization state, inline caches and GC pressure across
#      benchmarks within a process, so running them together made native
#      `centroid` measure 32,902 ns and then 236,497 ns on consecutive runs.
#
#   pwsh native.ps1

param([int]$Reps = 3, [switch]$Pin)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

function Run-Node([string[]]$argv) {
  $out = [System.IO.Path]::GetTempFileName()
  $p = Start-Process -PassThru -NoNewWindow -FilePath "node" `
       -ArgumentList $argv -RedirectStandardOutput $out
  if ($Pin) {
    Start-Sleep -Milliseconds 150
    try { $p.ProcessorAffinity = 1 } catch {}
  }
  $p.WaitForExit()
  Get-Content $out
  Remove-Item $out -Force
}

Run-Node @("native.mjs", "--check")
Write-Output ""

# THREE processes per case, because between-process variance on this machine is
# still large even pinned: the first process of a run measured G7 reuse at
# 138,026 ns and the next two at 92,871 and 94,496. One cold process is not a
# measurement either.
$cases = Run-Node @("native.mjs", "--list")
foreach ($c in $cases) {
  $c = $c.Trim()
  if ($c -eq "") { continue }
  foreach ($r in 1..$Reps) { Run-Node @("native.mjs", $c) }
}
