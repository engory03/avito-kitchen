param(
    [Parameter(Position=0)]
    [string]$Command = "help"
)

function Invoke-DockerCompose {
    param(
        [Parameter(ValueFromRemainingArguments = $true)]
        [string[]]$ComposeArgs
    )
    docker compose version *>$null
    if ($LASTEXITCODE -eq 0) {
        docker compose @ComposeArgs
    }
    else {
        docker-compose @ComposeArgs
    }
}

function Up {
    Write-Host "Starting Docker Compose stack..." -ForegroundColor Cyan
    Invoke-DockerCompose up -d --build
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

function Down {
    Write-Host "Stopping Docker Compose stack..." -ForegroundColor Cyan
    Invoke-DockerCompose down
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

function Seed {
    Write-Host "Re-sync demo menu (restart partner-demo)..." -ForegroundColor Cyan
    Invoke-DockerCompose restart partner-demo
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    Write-Host "Demo menu re-synced" -ForegroundColor Green
}

function Seed-Local {
    $repoRoot = if ([string]::IsNullOrEmpty($PSScriptRoot)) { (Get-Location).Path } else { $PSScriptRoot }
    Write-Host "Seeding catalog via partner-demo (local go run ./cmd/partner-demo; uses .env e.g. KITCHEN_BASE_URL=http://localhost:8080)..." -ForegroundColor Cyan
    Push-Location $repoRoot
    try {
        go run ./cmd/partner-demo
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    finally {
        Pop-Location
    }
}

function Invoke-GoTestSum {
    param(
        [string[]]$GoTestFlags = @(),
        [string[]]$Packages = @()
    )
    # PowerShell drops a bare `--` when calling functions; keep it in the argv array.
    $argv = @('tool', 'gotestsum', '--format', 'testname', '--') + $GoTestFlags + $Packages
    & go @argv
}

function Test-Project {
    $repoRoot = if ([string]::IsNullOrEmpty($PSScriptRoot)) { (Get-Location).Path } else { $PSScriptRoot }
    $coverageOut = Join-Path $repoRoot "coverage.out"
    Write-Host "Running tests with gotestsum (excludes ./e2e; use test-e2e for E2E)..." -ForegroundColor Cyan
    Write-Host "Coverage profile: $coverageOut" -ForegroundColor DarkGray
    Push-Location $repoRoot
    try {
        $pkgs = @(go list ./... | Where-Object { $_ -notmatch '/e2e$' -and $_ -notmatch '/docs/swagger$' })
        Invoke-GoTestSum -GoTestFlags @('-count=1', '-cover', "-coverprofile=$coverageOut", '-covermode=atomic') -Packages $pkgs
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        Write-Host ""
        Write-Host "Per-file summary (coverage.out):" -ForegroundColor Cyan
        go tool cover -func $coverageOut
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    finally {
        Pop-Location
    }
}

function Test-Integration {
    $repoRoot = if ([string]::IsNullOrEmpty($PSScriptRoot)) { (Get-Location).Path } else { $PSScriptRoot }
    $coverageOut = Join-Path $repoRoot "coverage-integration.out"
    Write-Host "Running integration-tagged tests with coverage (Docker / testcontainers required)..." -ForegroundColor Cyan
    Write-Host "Coverage profile: $coverageOut" -ForegroundColor DarkGray
    Push-Location $repoRoot
    try {
        $pkgs = @(go list ./... | Where-Object { $_ -notmatch '/e2e$' -and $_ -notmatch '/docs/swagger$' })
        Invoke-GoTestSum -GoTestFlags @('-count=1', '-timeout=8m', '-tags=integration', '-cover', "-coverprofile=$coverageOut", '-covermode=atomic') -Packages $pkgs
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        Write-Host ""
        Write-Host "Per-file summary (integration):" -ForegroundColor Cyan
        go tool cover -func $coverageOut
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    finally {
        Pop-Location
    }
}

function LoadTest {
    if (-not (Get-Command k6 -ErrorAction SilentlyContinue)) {
        Write-Host "Install k6: https://grafana.com/docs/k6/latest/set-up/install-k6/" -ForegroundColor Red
        exit 1
    }
    $repoRoot = if ([string]::IsNullOrEmpty($PSScriptRoot)) { (Get-Location).Path } else { $PSScriptRoot }
    $base = if ($env:BASE_URL) { $env:BASE_URL } else { "http://127.0.0.1:8080" }
    $rate = if ($env:LOADTEST_RATE) { $env:LOADTEST_RATE } else { "50" }
    $dur = if ($env:LOADTEST_DURATION) { $env:LOADTEST_DURATION } else { "90s" }
    Write-Host "k6 catalog load test: BASE_URL=$base RATE=$rate DURATION=$dur" -ForegroundColor Cyan
    Push-Location $repoRoot
    try {
        k6 run loadtest/k6/catalog.js -e "BASE_URL=$base" -e "LOADTEST_RATE=$rate" -e "LOADTEST_DURATION=$dur"
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    finally {
        Pop-Location
    }
}

function LoadTest-Smoke {
    if (-not (Get-Command k6 -ErrorAction SilentlyContinue)) {
        Write-Host "Install k6: https://grafana.com/docs/k6/latest/set-up/install-k6/" -ForegroundColor Red
        exit 1
    }
    $repoRoot = if ([string]::IsNullOrEmpty($PSScriptRoot)) { (Get-Location).Path } else { $PSScriptRoot }
    $base = if ($env:BASE_URL) { $env:BASE_URL } else { "http://127.0.0.1:8080" }
    Push-Location $repoRoot
    try {
        k6 run loadtest/k6/catalog.js -e "BASE_URL=$base" -e "LOADTEST_PROFILE=smoke"
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    finally {
        Pop-Location
    }
}

function LoadTest-Docker {
    $repoRoot = if ([string]::IsNullOrEmpty($PSScriptRoot)) { (Get-Location).Path } else { $PSScriptRoot }
    $rate = if ($env:LOADTEST_RATE) { $env:LOADTEST_RATE } else { "50" }
    $dur = if ($env:LOADTEST_DURATION) { $env:LOADTEST_DURATION } else { "90s" }
    Write-Host "k6 (Docker) against host.docker.internal:8080 ..." -ForegroundColor Cyan
    docker run --rm --add-host=host.docker.internal:host-gateway `
        -v "${repoRoot}:/t" -w /t `
        -e BASE_URL=http://host.docker.internal:8080 `
        -e LOADTEST_RATE=$rate `
        -e LOADTEST_DURATION=$dur `
        grafana/k6:0.57.0 run loadtest/k6/catalog.js
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

function Test-E2E {
    $repoRoot = if ([string]::IsNullOrEmpty($PSScriptRoot)) { (Get-Location).Path } else { $PSScriptRoot }
    if ($env:E2E_BASE_URL) {
        Write-Host "Running E2E tests against $($env:E2E_BASE_URL) ..." -ForegroundColor Cyan
    } else {
        Write-Host "Running E2E tests (testcontainers Postgres + in-process API; Docker required) ..." -ForegroundColor Cyan
    }
    Push-Location $repoRoot
    try {
        Invoke-GoTestSum -GoTestFlags @('-count=1', '-timeout=8m', '-v') -Packages @('./e2e/')
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    finally {
        Pop-Location
    }
}

function Lint {
    if (-not (Get-Command golangci-lint -ErrorAction SilentlyContinue)) {
        Write-Host "Install golangci-lint: https://golangci-lint.run/welcome/install/" -ForegroundColor Red
        exit 1
    }
    $repoRoot = if ([string]::IsNullOrEmpty($PSScriptRoot)) { (Get-Location).Path } else { $PSScriptRoot }
    Write-Host "Running linter..." -ForegroundColor Cyan
    Push-Location $repoRoot
    try {
        golangci-lint run ./...
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    finally {
        Pop-Location
    }
}

function Lint-Fix {
    if (-not (Get-Command golangci-lint -ErrorAction SilentlyContinue)) {
        Write-Host "Install golangci-lint: https://golangci-lint.run/welcome/install/" -ForegroundColor Red
        exit 1
    }
    $repoRoot = if ([string]::IsNullOrEmpty($PSScriptRoot)) { (Get-Location).Path } else { $PSScriptRoot }
    Write-Host "Running linter with autofix..." -ForegroundColor Cyan
    Push-Location $repoRoot
    try {
        golangci-lint run --fix ./...
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    finally {
        Pop-Location
    }
}

function Swagger {
    $repoRoot = if ([string]::IsNullOrEmpty($PSScriptRoot)) { (Get-Location).Path } else { $PSScriptRoot }
    Write-Host "Generating Swagger from godoc (swaggo) -> docs/swagger ..." -ForegroundColor Cyan
    Push-Location $repoRoot
    try {
        go tool swag init -g cmd/kitchen-api/main.go -o docs/swagger --parseInternal --exclude e2e
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        Write-Host "Swagger written to docs/swagger (UI: http://localhost:8080/swagger/index.html)" -ForegroundColor Green
    }
    finally {
        Pop-Location
    }
}

function Dev {
    $repoRoot = if ([string]::IsNullOrEmpty($PSScriptRoot)) { (Get-Location).Path } else { $PSScriptRoot }
    Write-Host "Starting kitchen-api (needs Postgres + Redis, e.g. .\tasks.ps1 up)..." -ForegroundColor Cyan
    Write-Host "Swagger UI: http://localhost:8080/swagger/index.html" -ForegroundColor DarkGray
    Push-Location $repoRoot
    try {
        go run ./cmd/kitchen-api
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    finally {
        Pop-Location
    }
}

function Build {
    $repoRoot = if ([string]::IsNullOrEmpty($PSScriptRoot)) { (Get-Location).Path } else { $PSScriptRoot }
    Write-Host "Building Go applications..." -ForegroundColor Cyan
    Push-Location $repoRoot
    try {
        if (-Not (Test-Path "bin")) {
            New-Item -ItemType Directory -Path "bin" | Out-Null
        }
        go build -o bin/kitchen-api.exe ./cmd/kitchen-api
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        go build -o bin/partner-demo.exe ./cmd/partner-demo
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        Write-Host "Build complete: bin/kitchen-api.exe bin/partner-demo.exe" -ForegroundColor Green
    }
    finally {
        Pop-Location
    }
}

function Run {
    $repoRoot = if ([string]::IsNullOrEmpty($PSScriptRoot)) { (Get-Location).Path } else { $PSScriptRoot }
    Write-Host "Running kitchen-api..." -ForegroundColor Cyan
    Write-Host "Swagger UI: http://localhost:8080/swagger/index.html" -ForegroundColor DarkGray
    Push-Location $repoRoot
    try {
        go run ./cmd/kitchen-api
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
    finally {
        Pop-Location
    }
}

function Help {
    Write-Host "Available commands:" -ForegroundColor Yellow
    Write-Host "  up             - Start project with dependencies (docker compose up -d --build)"
    Write-Host "  down           - Stop Docker Compose stack"
    Write-Host "  seed           - Re-upload demo menu (restart partner-demo)"
    Write-Host "  seed-local     - Run partner-demo locally (PUTs menu to kitchen-api)"
    Write-Host "  dev            - go run ./cmd/kitchen-api"
    Write-Host "  build          - Build kitchen-api and partner-demo into ./bin/"
    Write-Host "  run            - go run ./cmd/kitchen-api"
    Write-Host "  swag           - Generate docs/swagger from godoc (swaggo); UI at /swagger/index.html"
    Write-Host "  test           - gotestsum (go tool from go.mod) + coverage -> coverage.out"
    Write-Host "  test-integration - Integration tests (-tags=integration) + coverage -> coverage-integration.out"
    Write-Host "  test-e2e       - HTTP E2E (testcontainers by default; optional E2E_BASE_URL for external API)"
    Write-Host "  loadtest       - k6 GET /api/v1/restaurants (BASE_URL, LOADTEST_RATE, LOADTEST_DURATION)"
    Write-Host "  loadtest-smoke - Short k6 sanity run"
    Write-Host "  loadtest-docker - k6 via Docker + volume mount"
    Write-Host "  lint           - Run golangci-lint (see .golangci.yml)"
    Write-Host "  lint-fix       - Run golangci-lint with --fix"
    Write-Host "  help           - Show this help"
}

switch ($Command.ToLower()) {
    "up"    { Up }
    "down"  { Down }
    "seed"        { Seed }
    "seed-local"  { Seed-Local }
    "dev"   { Dev }
    "build" { Build }
    "run"   { Run }
    "swag"  { Swagger }
    "test"            { Test-Project }
    "test-integration" { Test-Integration }
    "test-e2e"        { Test-E2E }
    "loadtest"        { LoadTest }
    "loadtest-smoke"  { LoadTest-Smoke }
    "loadtest-docker" { LoadTest-Docker }
    "lint"            { Lint }
    "lint-fix"        { Lint-Fix }
    "help"            { Help }
    default {
        Write-Host "Unknown command: $Command" -ForegroundColor Red
        Help
    }
}
