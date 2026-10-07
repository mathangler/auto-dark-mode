@echo off
REM Build both release forms for Auto Dark Mode.
REM   build.bat             -> rebuild everything
REM   build.bat portable    -> only the single exe
REM   build.bat installer   -> only the NSIS guided installer
setlocal
set DIR=%~dp0
cd /d "%DIR%"

set ARG=%1
if "%ARG%"=="" set ARG=all

if /I "%ARG%"=="installer" goto installer
if /I "%ARG%"=="portable" goto portable

call :portable
call :installer
goto end

:portable
echo.
echo ==== Building portable single exe ====
call wails build || exit /b 1
echo Portable exe: build\bin\autodark.exe
goto :eof

:installer
echo.
echo ==== Building NSIS guided installer ====
call wails build -nsis || exit /b 1
echo Installer: build\bin\autodark-amd64-installer.exe
goto :eof

:end
echo.
echo Done.
endlocal
