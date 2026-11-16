# Every 10 s, the time, the CPU load, and every process not the benchmark's own
# that compiles or checks (another session's `go run`, a `check`, a `gen`, a
# clang): a timing taken beside one is suspect (gauntlet-2026-09-29 §5).
param([string]$Out, [string]$Stop)
while (-not (Test-Path $Stop)) {
    $load = (Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average).Average
    $others = Get-CimInstance Win32_Process | Where-Object {
        $_.Name -match '^(go|check|gen|clang|cmd|oro|javac)\.exe$' -or $_.CommandLine -match 'docs/book/check.go'
    } | ForEach-Object { "$($_.Name):$($_.ProcessId)" }
    "{0:o},{1},{2}" -f (Get-Date), $load, ($others -join ' ') | Add-Content $Out
    Start-Sleep -Seconds 10
}
