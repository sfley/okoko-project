@echo off
powershell.exe -NoProfile -File "%~dp0scripts\Deploy.ps1" %*
exit /b %errorlevel%
