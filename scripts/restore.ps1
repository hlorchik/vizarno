[CmdletBinding()]
param([Parameter(Mandatory)][string]$Backup, [switch]$LocalPostgres)
$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/postgres-tools.ps1"

$file = (Get-Item -LiteralPath $Backup).FullName
$target = "vizarno_restore_$runId"
$created = $false
try {
    if (!$LocalPostgres) {
        Invoke-Compose cp $file "db:$remoteFile"
        $file = $remoteFile
    }
    Invoke-Pg pg_restore --list $file | Out-Null
    Invoke-Pg createdb --template=template0 $target
    $created = $true
    Invoke-Pg pg_restore --exit-on-error --single-transaction --no-owner --no-acl -d $target $file
    Write-Output $target
} catch {
    if ($created) { Invoke-Pg dropdb $target }
    throw
} finally {
    if (!$LocalPostgres) { Invoke-Compose exec -T db rm -f $remoteFile }
}

