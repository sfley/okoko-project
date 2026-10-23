@echo off
powershell.exe -NoProfile -File "%~dp0scripts\Build.ps1" %*
exit /b %errorlevel%
