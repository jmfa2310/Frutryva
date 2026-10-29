# Run in a separate PowerShell process: pwsh -NoProfile -File scripts/test-update-windows.ps1
# Docker is mocked; synthetic fixture files are retained in a unique temp folder.
$ErrorActionPreference = "Stop"
$updater = Join-Path $PSScriptRoot "update-windows.ps1"
$fixture = Join-Path ([System.IO.Path]::GetTempPath()) ("futryva-update-test-" + [Guid]::NewGuid().ToString("N"))
$target = Join-Path $fixture "existing-installation"
New-Item -ItemType Directory -Path $target | Out-Null
$envFile = Join-Path $target ".env"
$originalEnv = "DB_PASSWORD=synthetic-fixture-only`nCOMPOSE_PROJECT_NAME=retained-project`nAPI_PORT=18080`n"
[System.IO.File]::WriteAllText($envFile, $originalEnv)
[System.IO.File]::WriteAllText((Join-Path $target "docker-compose.yml"), "original-compose-fixture")
[System.IO.File]::WriteAllText((Join-Path $target "user-note.txt"), "retained note")

$global:futryvaDockerCalls = @()
function global:docker {
    $global:futryvaDockerCalls += ($args -join " ")
    $global:LASTEXITCODE = 0
}

& $updater -ProjectPath $target
& $updater -ProjectPath $target

if ([System.IO.File]::ReadAllText($envFile) -cne $originalEnv) { throw "Existing .env changed" }
if ([System.IO.File]::ReadAllText((Join-Path $target "user-note.txt")) -ne "retained note") { throw "User file changed" }
if (Test-Path -LiteralPath (Join-Path $target ".git")) { throw "Git metadata was copied" }
$backups = @(Get-ChildItem -LiteralPath $fixture -Directory -Filter "FUTRYVA-code-backup-*")
if ($backups.Count -ne 2) { throw "Repeat update must retain distinct backups" }
$originalSaved = @($backups | Where-Object {
    [System.IO.File]::ReadAllText((Join-Path $_.FullName "docker-compose.yml")) -eq "original-compose-fixture"
})
if ($originalSaved.Count -ne 1) { throw "Original code backup not retained" }
if (@($global:futryvaDockerCalls | Where-Object { $_ -eq "compose up --build -d --wait" }).Count -ne 2) { throw "Expected safe startup commands" }
if (@($global:futryvaDockerCalls | Where-Object { $_ -match "down|prune|volume" }).Count) { throw "Destructive Docker command" }

# Missing API_PORT may be added, while every existing setting stays intact.
$withoutPort = $originalEnv.Replace("API_PORT=18080`n", "")
[System.IO.File]::WriteAllText($envFile, $withoutPort)
& $updater -ProjectPath $target
$updated = [System.IO.File]::ReadAllText($envFile)
if (-not $updated.StartsWith($withoutPort.TrimEnd())) { throw "Existing settings not preserved" }
if ($updated -notmatch '(?m)^API_PORT=18080$') { throw "Default API port missing" }
Write-Host "PASS: updater preserves .env, project name, user files and separate code backups; Docker mocked."
Write-Host "Synthetic fixtures retained: $fixture"
