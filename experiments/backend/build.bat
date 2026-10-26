@echo off
rem backend-2026-10-26: each benchmark printed from one IR_P, then built several
rem ways. Run from experiments\backend:   build.bat LLVM_BIN_DIR
rem   masm     ir/x86's file, assembled by ml64
rem   c        the prototype's C, compiled by cl -O2
rem   clangc   the same C, compiled by clang -O2
rem   llvm     the prototype's LLVM IR, compiled by clang -O2
rem   llvm-os  the same, -Os
rem   llvmsel  the prototype's LLVM IR with a two-continue branch as selects, -O2
rem Every object is linked with the same driver, which times gen_bench in-process.
setlocal enabledelayedexpansion
set "LLVM=%~1"
set "VCV="
for %%p in ("%ProgramFiles%\Microsoft Visual Studio" "%ProgramFiles(x86)%\Microsoft Visual Studio") do (
  for /d %%v in ("%%~p\*") do (
    for /d %%e in ("%%~v\*") do (
      if exist "%%~e\VC\Auxiliary\Build\vcvars64.bat" set "VCV=%%~e\VC\Auxiliary\Build\vcvars64.bat"
    )
  )
)
if not defined VCV (echo no MSVC & exit /b 1)
call "!VCV!" >nul
if not exist out mkdir out
for %%b in (sieve msort) do (
  go run ..\..\cmd\gen -targets ..\..\targets -path ..\..\lib -ir %%b.ir %%b.oro windows %%b.asm || exit /b 1
  go run .\print -lang llvm %%b.ir > %%b.ll || exit /b 1
  go run .\print -lang llvm -select %%b.ir > %%b-sel.ll || exit /b 1
  go run .\print -lang c %%b.ir > %%b.c || exit /b 1
  go run .\print -lang llvm -select -minsize %%b.ir > %%b-min.ll || exit /b 1
)
cl -nologo -O2 -c -Foout\driver.obj driver.c || exit /b 1
for %%b in (sieve msort) do (
  ml64 -nologo -c -Foout\%%b-masm.obj %%b.asm || exit /b 1
  cl -nologo -O2 -GS- -c -Foout\%%b-c.obj %%b.c || exit /b 1
  "%LLVM%\clang.exe" -O2 -c -o out\%%b-clangc.obj %%b.c || exit /b 1
  "%LLVM%\clang.exe" -O2 -c -o out\%%b-llvm.obj %%b.ll || exit /b 1
  "%LLVM%\clang.exe" -Os -c -o out\%%b-llvm-os.obj %%b.ll || exit /b 1
  "%LLVM%\clang.exe" -O2 -c -o out\%%b-llvmsel.obj %%b-sel.ll || exit /b 1
  "%LLVM%\clang.exe" -Oz -c -o out\%%b-llvmmin.obj %%b-min.ll || exit /b 1
  for %%k in (masm c clangc llvm llvm-os llvmsel llvmmin) do (
    link -nologo -out:out\%%b-%%k.exe out\driver.obj out\%%b-%%k.obj kernel32.lib || exit /b 1
  )
)
echo built
