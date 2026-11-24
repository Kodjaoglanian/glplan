# Glplan installer for Windows — one-line install:
#   irm https://raw.githubusercontent.com/Kodjaoglanian/glplan/main/install.ps1 | iex
param(
    [string]$InstallDir = "$env:LOCALAPPDATA\glplan\bin",
    [string]$ConfigDir  = "$env:USERPROFILE\.glplan"
)

$ErrorActionPreference = "Stop"
$Repo = "Kodjaoglanian/glplan"

function Write-Info($msg) { Write-Host "  [info] $msg" -ForegroundColor Cyan }
function Write-Ok($msg)   { Write-Host "  [ok]   $msg" -ForegroundColor Green }
function Write-Warn($msg) { Write-Host "  [warn] $msg" -ForegroundColor Yellow }
function Write-Err($msg)  { Write-Host "  [err]  $msg" -ForegroundColor Red; exit 1 }

Write-Host ""
Write-Host "  glplan — board kanban AI-first no terminal" -ForegroundColor DarkCyan
Write-Host ""

$arch = $env:PROCESSOR_ARCHITECTURE
switch ($arch) {
    "AMD64" { $target = "windows-amd64" }
    "ARM64" { $target = "windows-arm64" }
    default { Write-Err "Arquitetura não suportada: $arch" }
}
Write-Info "Alvo: $target"

if ($env:GLPLAN_VERSION) {
    $version = $env:GLPLAN_VERSION
} else {
    try {
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -Headers @{ "User-Agent" = "glplan-installer" }
        $version = $release.tag_name
    } catch {
        Write-Err "Falha ao consultar a última release: $_"
    }
}
if (-not $version) { Write-Err "Versão vazia; defina GLPLAN_VERSION" }
Write-Info "Versão: $version"

$archiveName = "glplan-$version-$target.zip"
$downloadUrl = "https://github.com/$Repo/releases/download/$version/$archiveName"
Write-Info "Baixando $archiveName"

$tmpDir = Join-Path $env:TEMP "glplan-install-$([guid]::NewGuid().ToString('N'))"
New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null
$archivePath = Join-Path $tmpDir $archiveName
try {
    Invoke-WebRequest -Uri $downloadUrl -OutFile $archivePath -UseBasicParsing
} catch {
    Write-Err "Download falhou: $_"
}

Write-Info "Verificando SHA256"
try {
    $shaPath = Join-Path $tmpDir "$archiveName.sha256"
    Invoke-WebRequest -Uri "$downloadUrl.sha256" -OutFile $shaPath -UseBasicParsing
    $expected = (Get-Content $shaPath -Raw).Trim().Split(" ")[0]
    $actual = (Get-FileHash $archivePath -Algorithm SHA256).Hash
    if ($actual -ne $expected) {
        Write-Err "Checksum inválido. Esperado: $expected; recebido: $actual"
    }
    Write-Ok "Checksum verificado"
} catch {
    Write-Warn "Arquivo .sha256 indisponível; pulando verificação"
}

Write-Info "Extraindo"
try {
    Expand-Archive -Path $archivePath -DestinationPath $tmpDir -Force
} catch {
    Write-Err "Falha ao extrair: $_"
}

$source = Join-Path $tmpDir "glplan.exe"
if (-not (Test-Path $source)) { Write-Err "glplan.exe não encontrado no arquivo" }

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null
$binaryPath = Join-Path $InstallDir "glplan.exe"
Move-Item $source $binaryPath -Force
Write-Ok "Instalado em $binaryPath"

$userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($userPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$InstallDir;$userPath", "User")
    Write-Ok "PATH do usuário atualizado"
} else {
    Write-Info "PATH já configurado"
}

Write-Host ""
try {
    $installedVersion = & $binaryPath --version
    Write-Ok "Glplan $installedVersion instalado com sucesso"
    Write-Host ""
    Write-Host "  glplan serve   inicia a API (um terminal)" -ForegroundColor Cyan
    Write-Host "  glplan tui     abre o board (outro terminal)" -ForegroundColor Cyan
    Write-Host "  glplan mcp     servidor MCP para agentes" -ForegroundColor Cyan
    Write-Host ""
    Write-Warn "reinicie o terminal para o PATH fazer efeito"
} catch {
    Write-Warn "instalação concluída, mas a verificação do binário falhou"
}

Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
