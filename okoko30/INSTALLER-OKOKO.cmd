@echo off
setlocal
set "OKOKO_INSTALL_ROOT=%~dp0"
set "OKOKO_INSTALL_ROUTER=%~1"
powershell.exe -NoLogo -NoProfile -Command "$ErrorActionPreference = 'Stop'; try { Write-Host '[1/5] Deblocage des fichiers OKOKO'; Get-ChildItem -LiteralPath $env:OKOKO_INSTALL_ROOT -Recurse -File | Unblock-File -ErrorAction Stop; & (Join-Path $env:OKOKO_INSTALL_ROOT 'scripts/Install.ps1') -Router $env:OKOKO_INSTALL_ROUTER; exit 0 } catch { Write-Host ('ERREUR : ' + $_.Exception.Message) -ForegroundColor Red; exit 1 }"
set "OKOKO_INSTALL_EXIT=%errorlevel%"
if not "%OKOKO_INSTALL_EXIT%"=="0" pause
exit /b %OKOKO_INSTALL_EXIT%
