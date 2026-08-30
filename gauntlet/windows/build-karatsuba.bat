@echo off
setlocal enabledelayedexpansion
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
ml64 -nologo -c -Fokaratsuba.obj karatsuba.asm || exit /b 1
link -nologo -subsystem:console -entry:main karatsuba.obj kernel32.lib -out:karatsuba.exe || exit /b 1
