@echo off
REM OmniLLM-Studio - Frontend Only
REM Starts the React/Vite frontend dev server

setlocal enabledelayedexpansion

set SCRIPT_DIR=%~dp0
set PROJECT_ROOT=%SCRIPT_DIR%..

echo.
echo Starting OmniLLM-Studio Frontend...
echo.

cd /d "%PROJECT_ROOT%\frontend"
npm run dev

pause
