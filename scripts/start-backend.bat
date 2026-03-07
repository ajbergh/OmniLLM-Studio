@echo off
REM OmniLLM-Studio - Backend Only
REM Starts the Go backend server

setlocal enabledelayedexpansion

set SCRIPT_DIR=%~dp0
set PROJECT_ROOT=%SCRIPT_DIR%..

echo.
echo Starting OmniLLM-Studio Backend...
echo.

cd /d "%PROJECT_ROOT%\backend"
go run ./cmd/server

pause
