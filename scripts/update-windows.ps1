param([string]$ProjectPath)

$ErrorActionPreference = "Stop"
$sourceRoot = Split-Path -Parent $PSScriptRoot

if (-not $ProjectPath) {
    Add-Type -AssemblyName System.Windows.Forms
    $picker = New-Object System.Windows.Forms.FolderBrowserDialog
    $picker.Description = "Seleciona a pasta da instalacao existente, onde estao .env e docker-compose.yml"
    if ($picker.ShowDialog() -ne "OK") { throw "Atualizacao cancelada." }
    $ProjectPath = $picker.SelectedPath
    $picker.Dispose()
}

$targetRoot = (Resolve-Path -LiteralPath $ProjectPath).Path
$envFile = Join-Path $targetRoot ".env"
if (-not (Test-Path -LiteralPath $envFile)) {
    throw "A pasta escolhida nao tem .env. Escolhe a instalacao existente; a atualizacao nao cria outra palavra-passe."
}
if (-not (Test-Path -LiteralPath (Join-Path $targetRoot "docker-compose.yml"))) {
    throw "A pasta escolhida nao tem docker-compose.yml."
}

if ($sourceRoot -ne $targetRoot) {
    $backup = Join-Path (Split-Path -Parent $targetRoot) ("FUTRYVA-code-backup-" + (Get-Date -Format "yyyyMMdd-HHmmss") + "-" + [Guid]::NewGuid().ToString("N").Substring(0, 8))
    $items = @(".dockerignore", ".env.example", ".gitattributes", ".github", ".gitignore", "AGENTS.md", "CODEX_PRIMEIRA_TAREFA.md", "README.md", "apps", "backend", "database", "docker-compose.yml", "docs", "scripts")
    foreach ($item in $items) {
        $source = Join-Path $sourceRoot $item
        if (-not (Test-Path -LiteralPath $source)) { continue }
        if (Test-Path -LiteralPath $source -PathType Leaf) {
            $files = @(Get-Item -LiteralPath $source -Force)
        } else {
            $files = @(Get-ChildItem -LiteralPath $source -Recurse -File -Force | Where-Object {
                $_.FullName -notmatch '[\\/](node_modules|dist|build|coverage|\.git|__pycache__)[\\/]' -and $_.Name -ne '.env' -and $_.Extension -ne '.log'
            })
        }
        foreach ($file in $files) {
            $relative = $file.FullName.Substring($sourceRoot.Length).TrimStart([char[]]"\/")
            $target = Join-Path $targetRoot $relative
            if (Test-Path -LiteralPath $target -PathType Leaf) {
                $old = Join-Path $backup $relative
                New-Item -ItemType Directory -Force -Path (Split-Path -Parent $old) | Out-Null
                Copy-Item -LiteralPath $target -Destination $old
            }
            New-Item -ItemType Directory -Force -Path (Split-Path -Parent $target) | Out-Null
            Copy-Item -LiteralPath $file.FullName -Destination $target -Force
        }
    }
    Write-Host "Backup do codigo anterior: $backup"
}

# Preserve all existing variables, especially database password and project name.
$settings = [System.IO.File]::ReadAllText($envFile)
if ($settings -notmatch '(?m)^\s*API_PORT\s*=') {
    $settings = $settings.TrimEnd() + "`nAPI_PORT=18080`n"
    [System.IO.File]::WriteAllText($envFile, $settings, (New-Object System.Text.UTF8Encoding($false)))
}
Set-Location -LiteralPath $targetRoot
docker compose config --quiet
if ($LASTEXITCODE -ne 0) { throw "Configuracao Docker invalida. O .env existente foi preservado." }
docker compose up --build -d --wait
if ($LASTEXITCODE -ne 0) {
    docker compose ps -a
    docker compose logs --tail=60 api manager
    throw "Falha no arranque. Envia os erros acima, sem partilhar .env."
}
docker compose ps
Write-Host "Atualizacao concluida. Abre o Manager no navegador: http://127.0.0.1:18081"
Write-Host "Emails de desenvolvimento: http://127.0.0.1:18025 (nao envia mensagens reais)."
Write-Host "As portas podem ser personalizadas por MANAGER_PORT, MAIL_PORT e API_PORT no .env."
