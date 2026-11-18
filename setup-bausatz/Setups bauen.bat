@echo off
setlocal enabledelayedexpansion
title Tri-Vertical - Setups bauen
cd /d "%~dp0"

rem Inno Setup suchen (Version 7 oder 6, fuer alle Benutzer oder nur fuer diesen Benutzer)
set "ISCC="
for %%P in (
  "%ProgramFiles%\Inno Setup 7\ISCC.exe"
  "%ProgramFiles(x86)%\Inno Setup 7\ISCC.exe"
  "%LOCALAPPDATA%\Programs\Inno Setup 7\ISCC.exe"
  "%ProgramFiles(x86)%\Inno Setup 6\ISCC.exe"
  "%ProgramFiles%\Inno Setup 6\ISCC.exe"
  "%LOCALAPPDATA%\Programs\Inno Setup 6\ISCC.exe"
) do if not defined ISCC if exist %%P set "ISCC=%%~P"

if not defined ISCC (
  echo.
  echo  Inno Setup wurde nicht gefunden.
  echo  Bitte Inno Setup 6 oder 7 installieren ^(innosetup-....exe in den Downloads^)
  echo  und diese Datei danach erneut starten.
  echo.
  pause
  exit /b 1
)
echo Verwende: %ISCC%
echo.

rem Download-Kennzeichen der Bausatz-Dateien entfernen (verhindert Windows-Warnungen)
powershell -NoProfile -Command "Get-ChildItem -LiteralPath '%~dp0' -Recurse -File | Unblock-File" >nul 2>&1

set FEHLER=0
set ANZAHL=0
for /r "%~dp0" %%S in (*.iss) do (
  echo === Baue %%~nxS ...
  pushd "%%~dpS"
  "%ISCC%" /Q "%%~nxS"
  if errorlevel 1 (
    echo     FEHLER beim Bauen von %%~nxS
    set FEHLER=1
  ) else (
    echo     fertig.
    set /a ANZAHL+=1
  )
  popd
)

echo.
if "%FEHLER%"=="0" (
  echo %ANZAHL% Setup^(s^) im Ordner "Setups" erstellt.
  start "" explorer "%~dp0Setups"
) else (
  echo Mindestens ein Setup konnte nicht gebaut werden - Meldungen oben beachten.
)
echo.
pause
