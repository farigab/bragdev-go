@echo off
REM run.bat - runs the BragDev server (Windows)
REM Usage: run from repository root: run.bat

where go >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [ERRO] Go nao encontrado no PATH.
    echo Instale em: https://golang.org/dl/
    exit /b 1
)

if not exist "cmd\bragdev" (
    echo [ERRO] Diretorio cmd\bragdev nao encontrado.
    echo Execute este script a partir da raiz do repositorio.
    exit /b 1
)

if exist .env (
    echo Carregando variaveis de .env...
    for /f "usebackq tokens=1,* delims== eol=#" %%A in (".env") do set %%A=%%B
) else (
    echo [AVISO] Arquivo .env nao encontrado. Usando variaveis do sistema.
)

echo Iniciando servidor BragDev...
go run ./cmd/bragdev %*

if %ERRORLEVEL% neq 0 (
    echo.
    echo [FALHA] O servidor encerrou com erro.
    exit /b 1
)
