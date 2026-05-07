@echo off
REM OmniLLM-Studio — Wails Dev Mode (Windows)
REM Starts the app in Wails dev mode with hot-reload.
REM Wails proxies the frontend Vite dev server and wraps it in a WebView2 window.
REM
REM Requirements:
REM   - Go 1.24+
REM   - Node.js 18+
REM   - Wails CLI v2: go install github.com/wailsapp/wails/v2/cmd/wails@latest
REM   - GCC (MinGW-w64) for CGO/SQLite
REM   - WebView2 runtime (ships with Win10 1803+)

setlocal enabledelayedexpansion

set SCRIPT_DIR=%~dp0
set PROJECT_ROOT=%SCRIPT_DIR%..

echo.
echo ==========================================
echo   OmniLLM-Studio — Wails Dev Mode
echo ==========================================
echo.

REM --- Check prerequisites ---
where wails >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Wails CLI not found. Install with:
    echo   go install github.com/wailsapp/wails/v2/cmd/wails@latest
    exit /b 1
)

where go >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Go not found in PATH.
    exit /b 1
)

where node >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Node.js not found in PATH.
    exit /b 1
)

REM --- Install frontend deps if needed ---
if not exist "%PROJECT_ROOT%\frontend\node_modules" (
    echo [setup] Installing frontend dependencies...
    cd /d "%PROJECT_ROOT%\frontend"
    call npm install --silent
)

REM --- Launch Wails dev ---
echo Starting Wails dev server...
echo   The app window will open automatically.
echo   Press Ctrl+C to stop.
echo.

cd /d "%PROJECT_ROOT%\backend\cmd\desktop"
wails dev -frontenddevserverurl http://localhost:5173

pause
