"""Times examples/hash against hand-written code (hashbench-2026-10-27).

    python -I experiments/hashbench/time.py DIR FILE

DIR holds the builds: fnv-go.exe, crc32-go.exe, handgo.exe, fnv-java/,
crc32-java/, handjava/, crc32.mjs (generated) and the repository's
experiments/hashbench/crc32.mjs (hand-written). Each program runs end to end,
reading FILE, serially and interleaved with its pair, five rounds; the best
wall time of each is reported, with the ratio generated / hand-written.
"""
import os, subprocess, sys, time

d, f = sys.argv[1], sys.argv[2]
here = os.path.dirname(os.path.abspath(__file__))
pairs = {
    'go fnv':     ([os.path.join(d, 'fnv-go.exe'), f], [os.path.join(d, 'handgo.exe'), 'fnv', f]),
    'go crc32':   ([os.path.join(d, 'crc32-go.exe'), f], [os.path.join(d, 'handgo.exe'), 'crc32', f]),
    'java fnv':   (['java', '-cp', os.path.join(d, 'fnv-java'), 'Main', f], ['java', '-cp', os.path.join(d, 'handjava'), 'Hash', 'fnv', f]),
    'java crc32': (['java', '-cp', os.path.join(d, 'crc32-java'), 'Main', f], ['java', '-cp', os.path.join(d, 'handjava'), 'Hash', 'crc32', f]),
    'js crc32':   (['node', os.path.join(d, 'crc32.mjs'), f], ['node', os.path.join(here, 'crc32.mjs'), f]),
}
extra = {'go lib (fnv32+fnv64+crc32)': [os.path.join(d, 'handgo.exe'), 'lib', f]}

def run(cmd):
    t = time.perf_counter()
    subprocess.run(cmd, check=True, stdout=subprocess.DEVNULL)
    return time.perf_counter() - t

best = {}
for _ in range(5):
    for name, (gen, hand) in pairs.items():
        for k, cmd in (('gen', gen), ('hand', hand)):
            best[(name, k)] = min(best.get((name, k), 1e9), run(cmd))
    for name, cmd in extra.items():
        best[(name, '')] = min(best.get((name, ''), 1e9), run(cmd))
for name in pairs:
    g, h = best[(name, 'gen')], best[(name, 'hand')]
    print(f'{name:12} generated {g*1000:8.1f} ms   hand-written {h*1000:8.1f} ms   ratio {g/h:.2f}')
for name in extra:
    print(f'{name:12} {best[(name, "")]*1000:8.1f} ms')
