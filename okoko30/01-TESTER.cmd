@echo off
powershell.exe -NoProfile -File "%~dp0scripts\Test.ps1" %*
exit /b %errorlevel%
