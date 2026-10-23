@echo off
powershell.exe -NoProfile -File "%~dp0scripts\Start.ps1" %*
exit /b %errorlevel%
