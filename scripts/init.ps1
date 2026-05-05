param(
    [switch]$Build
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Split-Path -Parent $PSScriptRoot
$isWindows = $env:OS -eq "Windows_NT"
$binaryName = if ($isWindows) { "amazon-crawler.exe" } else { "amazon-crawler" }

Push-Location $repoRoot
try {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "go command not found. Install Go 1.19+ and add it to PATH."
    }

    Write-Host "[1/3] Downloading Go modules..."
    & go mod download
    if ($LASTEXITCODE -ne 0) {
        throw "go mod download failed."
    }

    if (-not (Test-Path "config.yaml")) {
        if (-not (Test-Path "config.yaml.example")) {
            throw "Missing config template: config.yaml.example"
        }

        Copy-Item "config.yaml.example" "config.yaml"
        Write-Host "[2/3] Created config.yaml from config.yaml.example"
    } else {
        Write-Host "[2/3] Found existing config.yaml, skipping copy"
    }

    if ($Build) {
        Write-Host "[3/3] Building binary..."
        & go build -o $binaryName .
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed."
        }

        Write-Host "[done] Built $binaryName"
    } else {
        Write-Host "[3/3] Skipped build. Use -Build to generate the binary."
    }

    Write-Host ""
    Write-Host "Next steps:"
    Write-Host "1. Edit config.yaml and fill in basic, mysql, and proxy settings"
    Write-Host "2. Initialize database: mysql -u root -p < .\sql\ddl.sql"
    Write-Host "3. Import keywords: mysql -D taotie -u root -p < .\sql\category.sql"
    Write-Host "4. Insert the cookie record into amc_cookie for your host_id"
    Write-Host "5. Start with: go run . -c config.yaml"
    Write-Host "6. Or run the built binary: .\$binaryName -c config.yaml"
} finally {
    Pop-Location
}
