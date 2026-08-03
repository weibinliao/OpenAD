[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$findings = [System.Collections.Generic.List[string]]::new()

function Test-SafeDistinguishedName {
    param([string]$Value)

    $labels = @(
        [regex]::Matches($Value, '(?i)DC=([A-Za-z0-9-]+)') |
            ForEach-Object { $_.Groups[1].Value.ToLowerInvariant() }
    )
    if ($labels.Count -lt 2) {
        return $true
    }

    $suffix = ($labels | Select-Object -Last 2) -join '.'
    return $suffix -match '^(?:example\.(?:com|net|org)|[a-z0-9-]+\.test)$'
}

$rules = @(
    [pscustomobject]@{
        Name = 'RFC 1918 address literal'
        Pattern = '(?<!\d)(?:10(?:\.\d{1,3}){3}|192\.168(?:\.\d{1,3}){2}|172\.(?:1[6-9]|2\d|3[01])(?:\.\d{1,3}){2})(?!\d)'
        Options = [System.Text.RegularExpressions.RegexOptions]::None
    }
    [pscustomobject]@{
        Name = 'local or internal DNS suffix'
        Pattern = '(?i)(?<![A-Za-z0-9_.-])(?:[A-Za-z0-9-]+\.)+(?:local|internal|lan|corp)(?![A-Za-z0-9.-])'
        Options = [System.Text.RegularExpressions.RegexOptions]::None
    }
    [pscustomobject]@{
        Name = 'non-example AD distinguished name'
        Pattern = '(?i)(?:DC=[A-Za-z0-9-]+,?){2,}'
        Options = [System.Text.RegularExpressions.RegexOptions]::None
        Validator = { param([string]$Value) Test-SafeDistinguishedName -Value $Value }
    }
    [pscustomobject]@{
        Name = 'non-generic domain account literal'
        Pattern = '(?<![A-Za-z0-9_])(?!(?:EXAMPLE|DOMAIN|BUILTIN|AUTHORITY|APPDATA|PROGRAMDATA|SYSTEMROOT|TEMP|TMP)\\)[A-Z][A-Z0-9_-]{2,}\\[A-Za-z0-9_.-]+'
        Options = [System.Text.RegularExpressions.RegexOptions]::None
    }
    [pscustomobject]@{
        Name = 'local user or repository path'
        Pattern = '(?i)(?:\b[A-Z]:\\(?:Users|Documents and Settings)\\|\b[A-Z]:\\[^\r\n`]*\bOpenAD(?:[\\/.`]|$))'
        Options = [System.Text.RegularExpressions.RegexOptions]::None
    }
)

Push-Location $repoRoot
try {
    $textFiles = @(git grep -I -l -e '.')
    foreach ($relativePath in $textFiles) {
        $lines = @(Get-Content -LiteralPath $relativePath -Encoding utf8)
        foreach ($rule in $rules) {
            foreach ($line in $lines) {
                $matches = [regex]::Matches($line, $rule.Pattern, $rule.Options)
                foreach ($match in $matches) {
                    if ($rule.PSObject.Properties.Name -contains 'Validator' -and
                        -not (& $rule.Validator $match.Value)) {
                        $findings.Add("$relativePath [$($rule.Name)]")
                        break
                    }
                    if ($rule.PSObject.Properties.Name -notcontains 'Validator') {
                        $findings.Add("$relativePath [$($rule.Name)]")
                        break
                    }
                }
                if ($findings.Contains("$relativePath [$($rule.Name)]")) {
                    break
                }
            }
        }
    }

    $requiredConfig = @(
        @{ Path = '.env.example'; Pattern = '(?m)^APP_HOST=127\.0\.0\.1$'; Name = 'local APP_HOST example' }
        @{ Path = '.env.example'; Pattern = '(?m)^POSTGRES_PASSWORD=replace-with-a-strong-random-password$'; Name = 'database password placeholder' }
        @{ Path = '.env.example'; Pattern = '(?m)^CLICKHOUSE_PASSWORD=replace-with-a-strong-random-password$'; Name = 'analytics password placeholder' }
        @{ Path = '.env.example'; Pattern = '(?m)^REDIS_PASSWORD=replace-with-a-strong-random-password$'; Name = 'cache password placeholder' }
        @{ Path = '.env.example'; Pattern = '(?m)^JWT_SECRET=replace-with-a-generated-32-byte-secret$'; Name = 'JWT secret placeholder' }
    )
    foreach ($check in $requiredConfig) {
        $content = Get-Content -Raw -LiteralPath $check.Path -Encoding utf8
        if (-not [regex]::IsMatch($content, $check.Pattern)) {
            $findings.Add("$($check.Path) [$($check.Name)]")
        }
    }
}
finally {
    Pop-Location
}

if ($findings.Count -gt 0) {
    Write-Host 'Public-source audit failed:'
    $findings | Sort-Object -Unique | ForEach-Object { Write-Host " - $_" }
    exit 1
}

Write-Host "[OK] Public-source audit passed ($($textFiles.Count) text files checked)."
