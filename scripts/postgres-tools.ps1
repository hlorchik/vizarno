$projectRoot = Split-Path $PSScriptRoot -Parent
$runId = [guid]::NewGuid().ToString('N')
$remoteFile = "/tmp/vizarno-$runId.dump"
$database = 'vizarno'
if ($LocalPostgres -and $env:PGDATABASE) { $database = $env:PGDATABASE }

function Invoke-Compose {
    & docker compose --project-directory $projectRoot @args
    if ($LASTEXITCODE -ne 0) { throw "Docker Compose exited with code $LASTEXITCODE" }
}

function Invoke-Pg {
    param([string]$Tool)
    if ($LocalPostgres) {
        & $Tool @args
        if ($LASTEXITCODE -ne 0) { throw "$Tool exited with code $LASTEXITCODE" }
    } else {
        Invoke-Compose exec -T -e PGUSER=vizarno db $Tool @args
    }
}
