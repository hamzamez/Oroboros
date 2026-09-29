import csv, statistics, sys
from collections import defaultdict

# usage: analyze.py FILE.csv GEN=HAND[,HAND2...] ...
path, specs = sys.argv[1], sys.argv[2:]
v = defaultdict(list)
for row in csv.reader(open(path)):
    if len(row) != 4 or not row[3]:
        continue
    try:
        v[(row[1], row[2])].append(float(row[3]))
    except ValueError:
        pass

def med(tree, b):
    xs = v.get((tree, b))
    return statistics.median(xs) if xs else None

def spread(tree, b):
    xs = v.get((tree, b))
    if not xs or len(xs) < 2:
        return ''
    return '%.0f%%' % (100 * (max(xs) - min(xs)) / statistics.median(xs))

print('| generated | hand-written | n | IR (new) | ratio | term (old) | ratio | new/old |')
print('|---|---|---:|---:|---:|---:|---:|---:|')
for s in specs:
    gen, hands = s.split('~')
    for h in hands.split(','):
        gn, go_ = med('new', gen), med('old', gen)
        # the hand-written reference runs in BOTH trees: pool it, it is the same code
        hs = v.get(('new', h), []) + v.get(('old', h), [])
        hm = statistics.median(hs) if hs else None
        if gn is None or hm is None:
            print(f'| {gen} | {h} | missing |')
            continue
        rn = gn / hm
        ro = go_ / hm if go_ else float('nan')
        no = gn / go_ if go_ else float('nan')
        print(f'| {gen} | {h} | {len(v[("new", gen)])} | {gn:,.1f} ({spread("new", gen)}) | **{rn:.2f}×** | '
              f'{go_:,.1f} ({spread("old", gen)}) | {ro:.2f}× | {no:.2f} |')

# drift control: the hand-written code, same source, in the two trees
print()
print('| hand-written (drift control) | old tree | new tree | new/old |')
print('|---|---:|---:|---:|')
seen = set()
for s in specs:
    for h in s.split('~')[1].split(','):
        if h in seen:
            continue
        seen.add(h)
        a, b = med('old', h), med('new', h)
        if a and b:
            print(f'| {h} | {a:,.1f} | {b:,.1f} | {b / a:.2f} |')

# SUMMARY: ratios compose multiplicatively, so their mean is the GEOMETRIC mean
# (Fleming & Wallace 1986); one generated case per program, its first reference.
import math
rn, ro, no = [], [], []
for s in specs:
    gen, hands = s.split('~')
    h = hands.split(',')[0]
    hs = v.get(('new', h), []) + v.get(('old', h), [])
    gn, go_ = med('new', gen), med('old', gen)
    if gn and go_ and hs:
        hm = statistics.median(hs)
        rn.append(gn / hm); ro.append(go_ / hm); no.append(gn / go_)
g = lambda xs: math.exp(sum(map(math.log, xs)) / len(xs)) if xs else float('nan')
print()
print(f'geometric mean over {len(rn)}: IR/hand {g(rn):.3f}, term/hand {g(ro):.3f}, IR/term {g(no):.3f}; '
      f'IR/hand range {min(rn):.2f}-{max(rn):.2f}')
