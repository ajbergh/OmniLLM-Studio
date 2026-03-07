@echo off
REM OmniLLM-Studio — Wails Build Script for Windows
REM Produces: backend\cmd\desktop\build\bin\OmniLLM-Studio.exe
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
echo   OmniLLM-Studio — Build for Windows
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

REM --- Build frontend ---
echo [1/3] Building frontend...
cd /d "%PROJECT_ROOT%\frontend"
if exist node_modules (
    call npm install --silent
    if %errorlevel% neq 0 (
        echo [WARN] npm install failed. Retrying with a clean npm ci...
        call npm ci --silent
        if %errorlevel% neq 0 (
            echo [ERROR] Frontend dependency install failed.
            exit /b 1
        )
    )
) else (
    call npm ci --silent
    if %errorlevel% neq 0 (
        echo [WARN] npm ci failed. Retrying with npm install...
        call npm install --silent
        if %errorlevel% neq 0 (
            echo [ERROR] Frontend dependency install failed.
            exit /b 1
        )
    )
)
call npm run build
if %errorlevel% neq 0 (
    echo [ERROR] Frontend build failed.
    exit /b 1
)

REM --- Copy frontend dist to desktop embed directory ---
echo [2/3] Embedding frontend assets...
set EMBED_DIR=%PROJECT_ROOT%\backend\cmd\desktop\frontend_dist
if exist "%EMBED_DIR%" rmdir /s /q "%EMBED_DIR%"
xcopy /e /i /q "%PROJECT_ROOT%\frontend\dist" "%EMBED_DIR%" >nul
if %errorlevel% neq 0 (
    echo [ERROR] Failed to copy frontend assets.
    exit /b 1
)

REM --- Build with Wails ---
echo [3/3] Building Windows binary with Wails...
cd /d "%PROJECT_ROOT%\backend\cmd\desktop"

REM Get version from git tag or default
for /f "tokens=*" %%i in ('git describe --tags --always 2^>nul') do set GIT_VERSION=%%i
if "%GIT_VERSION%"=="" set GIT_VERSION=dev

for /f "tokens=*" %%i in ('git rev-parse --short HEAD 2^>nul') do set GIT_COMMIT=%%i
if "%GIT_COMMIT%"=="" set GIT_COMMIT=unknown

wails build -s -clean -trimpath -platform windows/amd64 -ldflags "-X main.version=%GIT_VERSION% -X main.commit=%GIT_COMMIT%"
if %errorlevel% neq 0 (
    echo [ERROR] Wails build failed.
    exit /b 1
)

REM --- Copy output to project build directory ---
if not exist "%PROJECT_ROOT%\build\bin" mkdir "%PROJECT_ROOT%\build\bin"
copy /y "%PROJECT_ROOT%\backend\cmd\desktop\build\bin\OmniLLM-Studio.exe" "%PROJECT_ROOT%\build\bin\OmniLLM-Studio.exe" >nul

echo.
echo ==========================================
echo   Build complete!
echo   Output: build\bin\OmniLLM-Studio.exe
echo ==========================================
echo.
