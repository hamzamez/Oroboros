#!/usr/bin/env bash
# The four hosts in sequence, nothing of this session's running beside them: Go,
# JavaScript, Java, windows. sampler.ps1 logs the load and any other session's
# compiler or checker every 10 s, so a row timed beside one can be found.
D=$(cd "$(dirname "$0")" && pwd)
rm -f "$D/stop"
powershell -NoProfile -File "$(cygpath -w $D/sampler.ps1)" -Out "$(cygpath -w $D/procs.log)" -Stop "$(cygpath -w $D/stop)" &
for h in go js java win; do
  echo "$h start $(date -Iseconds)" >> $D/phases.log
  bash $D/${h}bench.sh > $D/$h.csv 2> $D/$h.err
done
echo "end $(date -Iseconds)" >> $D/phases.log
touch $D/stop
wait
echo done
