# The file programs' table (files-2026-11-16): each (program, variant)'s median
# wall time over the rounds, its interquartile spread, and the ratios. A
# process's time is its floor, the toolchain's own do-nothing process, plus
# its work, so the work is the median less that floor, and each ratio is
# given both ways: whole processes, and work alone.
#
#   python analyze.py runs.csv
import csv, statistics, sys
from collections import defaultdict

v = defaultdict(list)
for r in csv.reader(open(sys.argv[1])):
    if len(r) == 4:
        v[(r[1], r[2])].append(float(r[3]))

def med(p, var):
    xs = v.get((p, var))
    return statistics.median(xs) if xs else None

def iqr(p, var):
    xs = sorted(v.get((p, var), []))
    if len(xs) < 4:
        return ''
    q = statistics.quantiles(xs, n=4)
    return '%.1f–%.1f' % (q[0], q[2])

# the floor each variant is measured above: its toolchain's do-nothing process
floor_of = {'windows': 'windows', 'winold': 'windows', 'c': 'c', 'gohand': 'gohand', 'go': 'go'}

print('| program | variant | n | median ms | IQR | work ms (less its floor) |')
print('|---|---|---:|---:|---|---:|')
for p in ['nop', 'roundtrip', 'wc', 'count', 'freq', 'jsonfmt']:
    for var in ['windows', 'winold', 'c', 'gohand', 'go']:
        m = med(p, var)
        if m is None:
            continue
        fl = med('nop', floor_of[var])
        work = m - fl if fl is not None and p != 'nop' else None
        ws = '%.2f' % work if work is not None else ''
        print('| %s | %s | %d | %.2f | %s | %s |' % (p, var, len(v[(p, var)]), m, iqr(p, var), ws))

print()
print('| program | windows / C | windows / Go by hand | windows / the language on Go | work: windows / C | work: windows / Go by hand | windows new / old |')
print('|---|---:|---:|---:|---:|---:|---:|')
for p in ['roundtrip', 'wc', 'count', 'freq', 'jsonfmt']:
    w = med(p, 'windows')
    if w is None:
        continue
    def r(a, b):
        return '%.2f' % (a / b) if a is not None and b else '—'
    def work(var):
        m, fl = med(p, var), med('nop', floor_of[var])
        return m - fl if m is not None and fl is not None else None
    ww = work('windows')
    print('| %s | %s | %s | %s | %s | %s | %s |' % (p, r(w, med(p, 'c')), r(w, med(p, 'gohand')), r(w, med(p, 'go')),
          r(ww, work('c')), r(ww, work('gohand')), r(w, med(p, 'winold'))))
