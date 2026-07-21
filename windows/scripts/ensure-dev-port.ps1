param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('frontend', 'backend')]
    [string]$Role,

    [Parameter(Mandatory = $true)]
    [int]$Port,

    [Parameter(Mandatory = $true)]
    [string]$RepoRoot,

    [switch]$StopProjectOwned
)

$ErrorActionPreference = 'Stop'

function Get-ListeningProcessIds {
    @(Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue |
        Select-Object -ExpandProperty OwningProcess -Unique)
}

function Get-ListeningProcess([int]$ProcessId) {
    Get-CimInstance Win32_Process -Filter "ProcessId=$ProcessId" -ErrorAction SilentlyContinue
}

$normalizedRepoRoot = $RepoRoot.TrimEnd('\\')
$repoRootPattern = [regex]::Escape($normalizedRepoRoot)
$repoNamePattern = [regex]::Escape([System.IO.Path]::GetFileName($normalizedRepoRoot))

function Test-ProjectOwned([object]$Process) {
    if (-not $Process -or [string]::IsNullOrWhiteSpace($Process.CommandLine)) {
        return $false
    }

    $commandLine = $Process.CommandLine
    $belongsToRepo = $commandLine -match $repoRootPattern -or $commandLine -match $repoNamePattern
    if (-not $belongsToRepo) {
        return $false
    }

    switch ($Role) {
        'frontend' {
            return $commandLine -match 'next(\.cmd)?(\s|$)' -or
                $commandLine -match 'next\\dist\\bin\\next' -or
                $commandLine -match 'start-server\.js' -or
                $commandLine -match 'run-web-dev\.bat' -or
                $commandLine -match 'npm-cli\.js.*run dev'
        }
        'backend' {
            return $commandLine -match 'cmd/api' -or
                $commandLine -match 'api\.exe' -or
                $commandLine -match 'tools\\go\\bin\\go\.exe'
        }
    }

    return $false
}

function Format-State([string]$Prefix, [int]$ProcessId, [object]$Process) {
    $name = if ($Process -and $Process.Name) { $Process.Name } else { 'unknown' }
    return ('{0}:{1}:{2}' -f $Prefix, $ProcessId, $name)
}

function Test-BelongsToRepo([object]$Process) {
    if (-not $Process -or [string]::IsNullOrWhiteSpace($Process.CommandLine)) {
        return $false
    }

    return $Process.CommandLine -match $repoRootPattern -or $Process.CommandLine -match $repoNamePattern
}

function Stop-ProjectProcessTree([object]$Process) {
    $visited = New-Object 'System.Collections.Generic.HashSet[int]'
    $current = $Process

    while ($current -and $visited.Add([int]$current.ProcessId)) {
        try {
            Stop-Process -Id $current.ProcessId -Force -ErrorAction Stop
        }
        catch {
        }

        $parentId = [int]$current.ParentProcessId
        if ($parentId -le 0) {
            break
        }

        $parent = Get-ListeningProcess -ProcessId $parentId
        if (-not (Test-BelongsToRepo -Process $parent)) {
            break
        }

        $current = $parent
    }
}

function Stop-ProjectRoleProcesses {
    $repoProcesses = @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue)
    foreach ($candidate in $repoProcesses) {
        if (-not (Test-ProjectOwned -Process $candidate)) {
            continue
        }

        try {
            Stop-Process -Id $candidate.ProcessId -Force -ErrorAction Stop
        }
        catch {
        }
    }
}

$listeningProcessIds = Get-ListeningProcessIds
if ($listeningProcessIds.Count -eq 0) {
    Write-Output 'FREE'
    exit 0
}

foreach ($processId in $listeningProcessIds) {
    $process = Get-ListeningProcess -ProcessId $processId
    if (-not (Test-ProjectOwned -Process $process)) {
        Write-Output (Format-State -Prefix 'BUSY' -ProcessId $processId -Process $process)
        exit 0
    }

    if (-not $StopProjectOwned) {
        Write-Output (Format-State -Prefix 'PROJECT' -ProcessId $processId -Process $process)
        exit 0
    }

    try {
        Stop-ProjectProcessTree -Process $process
    }
    catch {
        Write-Output (Format-State -Prefix 'PROJECT' -ProcessId $processId -Process $process)
        exit 0
    }
}

Stop-ProjectRoleProcesses

Start-Sleep -Milliseconds 750

$remainingProcessIds = Get-ListeningProcessIds
if ($remainingProcessIds.Count -eq 0) {
    Write-Output 'FREE'
    exit 0
}

foreach ($processId in $remainingProcessIds) {
    $process = Get-ListeningProcess -ProcessId $processId
    if (Test-ProjectOwned -Process $process) {
        Write-Output (Format-State -Prefix 'PROJECT' -ProcessId $processId -Process $process)
        exit 0
    }

    Write-Output (Format-State -Prefix 'BUSY' -ProcessId $processId -Process $process)
    exit 0
}

Write-Output 'FREE'
