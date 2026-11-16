#!/usr/bin/env bash
# The file programs as processes, every variant (bench.go), the sampler beside.
D=$(cd "$(dirname "$0")" && pwd)
rm -f $D/stop
powershell -NoProfile -File "$(cygpath -w $D/sampler.ps1)" -Out "$(cygpath -w $D/procs.log)" -Stop "$(cygpath -w $D/stop)" &
cd $D/../../.. && go run gauntlet/files/bench.go "$(cygpath -w /c/Users/hamza/AppData/Local/Temp/claude/a27858592609c5af/d57e0ed4-2363-4c51-a8f2-f1d9ee6673b6/scratchpad/files/bin)" "$(cygpath -w /c/Users/hamza/AppData/Local/Temp/claude/a27858592609c5af/d57e0ed4-2363-4c51-a8f2-f1d9ee6673b6/scratchpad/files/in)" ${ROUNDS:-101} > $D/runs.csv 2> $D/runs.err
touch $D/stop
wait
cat $D/runs.err
