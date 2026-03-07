@echo off
REM OmniLLM-Studio - Development Launch Script
REM Starts both backend (Go) and frontend (React) servers

setlocal enabledelayedexpansion

REM Get the script directory
set SCRIPT_DIR=%~dp0
REM Go up one level to get the project root
set PROJECT_ROOT=%SCRIPT_DIR%..

echo.
echo ================================
echo   OmniLLM-Studio - Dev Launch
echo ================================
echo.

REM Start Backend
echo [1/2] Starting backend (Go)...
start "OmniLLM-Studio Backend" cmd /k "cd /d "%PROJECT_ROOT%\backend" && go run ./cmd/server"
timeout /t 2 /nobreak

REM Start Frontend
echo [2/2] Starting frontend (React + Vite)...
start "OmniLLM-Studio Frontend" cmd /k "cd /d "%PROJECT_ROOT%\frontend" && npm run dev"
timeout /t 2 /nobreak

echo.
echo ================================
echo   Servers Started
echo ================================
echo.
echo Backend:  http://localhost:8080
echo Frontend: http://localhost:5173
echo.
echo Press Ctrl+C in either window to stop that server.
echo.
pause
