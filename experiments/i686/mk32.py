"""Make the i686 experiment's targets: a copy of targets/ for i686-pc-windows-msvc.

    python -I experiments/i686/mk32.py OUT

writes OUT/targets and OUT/build32.bat. Every change below is the experiment's,
not a decision (docs/triple-research.md): the triple; ILP32's sets for the four
kernel32 functions whose SIZE_T the 64-bit declarations state as the word; the
address space's bound; a trap for a byte count past the 32-bit SIZE_T, where
the decision has no fact to prove it (§4.3); and the build line, which must name
the triple to clang (§2 item 1).
"""
import os
import shutil
import sys

out = os.path.abspath(sys.argv[1])
root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
tg = os.path.join(out, 'targets')
if os.path.exists(tg):
    shutil.rmtree(tg)
shutil.copytree(os.path.join(root, 'targets'), tg)


def edit(path, old, new):
    p = os.path.join(tg, path)
    s = open(p, encoding='utf-8', newline='').read()
    assert s.count(old) == 1, (path, old)
    open(p, 'w', encoding='utf-8', newline='').write(s.replace(old, new, 1))


edit('windows/windows.oro', 'triple "x86_64-pc-windows-msvc"', 'triple "i686-pc-windows-msvc"')
edit('windows/windows.oro', 'fact max_len (a ∈ Array(A)) len(a) <= (2^47-9)',
     'fact max_len (a ∈ Array(A)) len(a) <= (2^31-9)')
bat = os.path.join(out, 'build32.bat')
edit('windows/windows.oro', r'build "cmd /c .\\build.bat"',
     'build "cmd /c ' + bat.replace('/', '\\').replace('\\', '\\\\') + ' . main"')
for f in ['VirtualAlloc(a ∈ ptr, n ∈ 0..2^63-1', 'HeapAlloc(h ∈ ptr, f ∈ 0..2^32-1, n ∈ 0..2^63-1',
          'VirtualFree(a ∈ ptr, n ∈ 0..2^63-1', 'RtlZeroMemory(p ∈ ptr, n ∈ 0..2^63-1']:
    edit('windows/kernel32.oro', f, f.replace('0..2^63-1', '0..2^32-1'))
edit('windows/os.oro', 'fn max_len() (0..2^63-1) { 140737488355319 }', 'fn max_len() (0..2^63-1) { 2147483639 }')
for name, flags in [('region', '8'), ('region_raw', '0')]:
    edit('windows/os.oro', '    fn %s(b ∈ 0..2^63-1) (ptr) {\n        p = if b >= 65536' % name,
         '    fn %s(b ∈ 0..2^63-1) (ptr) {\n        p = if b > 2^32 - 1 { llvm.null() } else if b >= 65536' % name)
edit('windows/os.oro', '        z = kernel32.RtlZeroMemory(p, b)\n        true',
     '        if b > 2^32 - 1 { false } else { z = kernel32.RtlZeroMemory(p, b); true }')

# lib/os/windows.oro reads a file into one table of its size, guarded at
# 2^40 bytes, x64's; the experiment's copy guards at the 32-bit space's bound
lib = os.path.join(out, 'lib')
if os.path.exists(lib):
    shutil.rmtree(lib)
shutil.copytree(os.path.join(root, 'lib'), lib)
p = os.path.join(lib, 'os', 'windows.oro')
s = open(p, encoding='utf-8', newline='').read()
assert s.count('sz > 2^40') == 1 and s.count('sz ∈ 0..2^40') == 1
s = s.replace('sz > 2^40', 'sz > 2^31 - 9').replace('sz ∈ 0..2^40', 'sz ∈ 0..2^31-9')
open(p, 'w', encoding='utf-8', newline='').write(s)

vs = r'C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Auxiliary\Build\vcvars32.bat'
open(bat, 'w', newline='\r\n').write('\n'.join([
    '@echo off',
    'setlocal enabledelayedexpansion',
    'rem build32.bat DIR ENTRY: DIR\\main.ll to DIR\\main.exe for i686-pc-windows-msvc',
    'cd /d "%~1" || exit /b 1',
    'call "' + vs + '" >nul || exit /b 1',
    'set "CLANG=%LOCALAPPDATA%\\Programs\\LLVM\\bin\\clang.exe"',
    '"!CLANG!" --target=i686-pc-windows-msvc -O2 -Wno-override-module -c -o main.obj main.ll || exit /b 1',
    'link -nologo -subsystem:console -entry:%2 main.obj kernel32.lib -out:main.exe || exit /b 1',
    '']))
print('wrote', tg, lib, 'and', bat)
