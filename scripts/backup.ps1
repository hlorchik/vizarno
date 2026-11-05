[CmdletBinding()]
param([switch]$LocalPostgres)
$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/postgres-tools.ps1"

$folder = Join-Path $projectRoot 'backups'
[void](New-Item -ItemType Directory -Force -Path $folder)
$dest = Join-Path $folder ("vizarno-{0}-{1}.dump" -f (Get-Date -Format yyyyMMdd-HHmmss), $runId)
$partial = "$dest.partial"
try {
    $argsDump = @('-Fc', '--exclude-table-data=public.sessions', '-d', $database)
    if ($LocalPostgres) {
        Invoke-Pg pg_dump @argsDump -f $partial
        Invoke-Pg pg_restore --list $partial | Out-Null
    } else {
        Invoke-Pg pg_dump @argsDump -f $remoteFile
        Invoke-Pg pg_restore --list $remoteFile | Out-Null
        Invoke-Compose cp "db:$remoteFile" $partial
    }
    [IO.File]::Move($partial, $dest)
    Write-Output $dest
} finally {
    if (Test-Path -LiteralPath $partial) { Remove-Item -LiteralPath $partial }
    if (!$LocalPostgres) { Invoke-Compose exec -T db rm -f $remoteFile }
}
