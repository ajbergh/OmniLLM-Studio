@echo off
REM OmniLLM-Studio — Web Server Build for Windows (non-Wails)
REM Produces: build\web\omnillm-studio[-arm64].exe + build\web\frontend\
REM
REM This builds the standalone Go web server and the React frontend.
REM The server exposes the REST/SSE API on :8080.  Serve the frontend
REM separately (nginx, Caddy, or `npx serve`).
REM
REM Requirements:
REM   - Go 1.24+
REM   - Node.js 18+
REM   - GCC (MinGW-w64) for CGO/SQLite
REM
REM Usage:
REM   build-web-windows.bat           -> Windows x64 (amd64)
REM   build-web-windows.bat arm64     -> Windows ARM64

setlocal enabledelayedexpansion

set SCRIPT_DIR=%~dp0
set PROJECT_ROOT=%SCRIPT_DIR%..

REM --- Architecture ---
set ARCH=%1
if "%ARCH%"=="" set ARCH=amd64
if /i "%ARCH%"=="x64" set ARCH=amd64
if /i "%ARCH%"=="x86_64" set ARCH=amd64

echo.
echo ==========================================
echo   OmniLLM-Studio — Web Build (Windows)
echo   Architecture: %ARCH%
echo ==========================================
echo.

REM --- Check prerequisites ---
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
) else (
    call npm ci --silent
)
if %errorlevel% neq 0 (
    echo [ERROR] Frontend dependency install failed.
    exit /b 1
)
call npm run build
if %errorlevel% neq 0 (
    echo [ERROR] Frontend build failed.
    exit /b 1
)

REM --- Build Go web server ---
echo [2/3] Building Go web server...
cd /d "%PROJECT_ROOT%\backend"

for /f "tokens=*" %%i in ('git describe --tags --always 2^>nul') do set GIT_VERSION=%%i
if "%GIT_VERSION%"=="" set GIT_VERSION=dev

for /f "tokens=*" %%i in ('git rev-parse --short HEAD 2^>nul') do set GIT_COMMIT=%%i
if "%GIT_COMMIT%"=="" set GIT_COMMIT=unknown

if /i "%ARCH%"=="arm64" (
    set OUTPUT_NAME=omnillm-studio-arm64.exe
) else (
    set OUTPUT_NAME=omnillm-studio.exe
)

set CGO_ENABLED=1
set GOOS=windows
set GOARCH=%ARCH%

go build -trimpath -ldflags "-s -w -X main.version=%GIT_VERSION% -X main.commit=%GIT_COMMIT%" ^
    -o "%PROJECT_ROOT%\build\web\%OUTPUT_NAME%" ^
    ./cmd/server
if %errorlevel% neq 0 (
    echo [ERROR] Go build failed.
    exit /b 1
)

REM --- Copy frontend dist ---
echo [3/3] Copying frontend assets...
set WEB_FRONTEND=%PROJECT_ROOT%\build\web\frontend
if exist "%WEB_FRONTEND%" rmdir /s /q "%WEB_FRONTEND%"
xcopy /e /i /q "%PROJECT_ROOT%\frontend\dist" "%WEB_FRONTEND%" >nul
if %errorlevel% neq 0 (
    echo [ERROR] Failed to copy frontend assets.
    exit /b 1
)

echo.
echo ==========================================
echo   Web build complete!
echo   Server: build\web\%OUTPUT_NAME%
echo   Frontend: build\web\frontend\
echo.
echo   Run: build\web\%OUTPUT_NAME%
echo   Serve frontend separately (nginx / Caddy /
echo     npx serve build\web\frontend)
echo ==========================================
echo.
