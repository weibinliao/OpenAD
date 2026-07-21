param(
    [Parameter(Mandatory = $true)][string]$Root,
    [int]$Port = 3010,
    [string]$HostName = 'localhost'
)

$ErrorActionPreference = 'Stop'

$resolvedRoot = (Resolve-Path $Root).Path
$resolvedRootWithSeparator = $resolvedRoot.TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
$prefixHost = $HostName.Trim()
if ([string]::IsNullOrWhiteSpace($prefixHost)) {
    $prefixHost = 'localhost'
}
if ($prefixHost -eq '*' -or $prefixHost -eq '0.0.0.0') {
    $prefixHost = '+'
}

$listeners = @()

function Add-Listener([System.Net.IPAddress]$address) {
    $listener = [System.Net.Sockets.TcpListener]::new($address, $Port)
    $listener.Start()
    $script:listeners += $listener
}

if ($prefixHost -eq 'localhost') {
    Add-Listener ([System.Net.IPAddress]::Loopback)
    try { Add-Listener ([System.Net.IPAddress]::IPv6Loopback) } catch { }
} elseif ($prefixHost -eq '+') {
    try {
        $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::IPv6Any, $Port)
        $listener.Server.DualMode = $true
        $listener.Start()
        $listeners += $listener
    } catch {
        Add-Listener ([System.Net.IPAddress]::Any)
    }
} else {
    $address = $null
    if ([System.Net.IPAddress]::TryParse($prefixHost, [ref]$address)) {
        Add-Listener $address
    } else {
        $address = [System.Net.Dns]::GetHostAddresses($prefixHost) | Where-Object { $_.AddressFamily -eq [System.Net.Sockets.AddressFamily]::InterNetwork } | Select-Object -First 1
        if ($null -eq $address) {
            throw "Unable to resolve host name: $prefixHost"
        }
        Add-Listener $address
    }
}

Write-Host "Static web server listening at http://$prefixHost`:$Port"
Write-Host "Serving files from: $resolvedRoot"

function Get-ContentType([string]$path) {
    switch ([System.IO.Path]::GetExtension($path).ToLowerInvariant()) {
        '.html' { 'text/html; charset=utf-8' }
        '.js' { 'application/javascript; charset=utf-8' }
        '.css' { 'text/css; charset=utf-8' }
        '.json' { 'application/json; charset=utf-8' }
        '.svg' { 'image/svg+xml' }
        '.png' { 'image/png' }
        '.jpg' { 'image/jpeg' }
        '.jpeg' { 'image/jpeg' }
        '.ico' { 'image/x-icon' }
        '.map' { 'application/json; charset=utf-8' }
        default { 'application/octet-stream' }
    }
}

function Write-Response([System.Net.Sockets.NetworkStream]$stream, [int]$statusCode, [string]$reason, [string]$contentType, [byte[]]$body) {
    if ($null -eq $body) {
        $body = [byte[]]::new(0)
    }

    $headers = "HTTP/1.1 $statusCode $reason`r`nContent-Length: $($body.Length)`r`nConnection: close`r`n"
    if (-not [string]::IsNullOrWhiteSpace($contentType)) {
        $headers += "Content-Type: $contentType`r`n"
    }
    $headers += "`r`n"

    $headerBytes = [System.Text.Encoding]::ASCII.GetBytes($headers)
    $stream.Write($headerBytes, 0, $headerBytes.Length)
    if ($body.Length -gt 0) {
        $stream.Write($body, 0, $body.Length)
    }
}

function Handle-Client([System.Net.Sockets.TcpClient]$client) {
    try {
        $stream = $client.GetStream()
        $reader = [System.IO.StreamReader]::new($stream, [System.Text.Encoding]::ASCII, $false, 1024, $true)
        $requestLine = $reader.ReadLine()
        while ($null -ne ($line = $reader.ReadLine()) -and $line -ne '') { }

        if ([string]::IsNullOrWhiteSpace($requestLine) -or $requestLine -notmatch '^(GET|HEAD)\s+([^\s]+)\s+HTTP/') {
            Write-Response $stream 400 'Bad Request' 'text/plain; charset=utf-8' ([System.Text.Encoding]::UTF8.GetBytes('Bad Request'))
            return
        }

        $method = $Matches[1]
        $requestTarget = $Matches[2].Split('?')[0]
        $requestPath = [System.Uri]::UnescapeDataString($requestTarget.TrimStart('/'))
        if ([string]::IsNullOrWhiteSpace($requestPath)) {
            $requestPath = 'index.html'
        }

        $candidatePath = Join-Path $resolvedRoot $requestPath
        if ((Test-Path $candidatePath) -and (Get-Item $candidatePath).PSIsContainer) {
            $candidatePath = Join-Path $candidatePath 'index.html'
        }

        if (-not (Test-Path $candidatePath)) {
            $fallback = Join-Path $resolvedRoot 'index.html'
            if (Test-Path $fallback) {
                $candidatePath = $fallback
            } else {
                Write-Response $stream 404 'Not Found' 'text/plain; charset=utf-8' ([System.Text.Encoding]::UTF8.GetBytes('Not Found'))
                return
            }
        }

        $resolvedCandidate = (Resolve-Path $candidatePath).Path
        if ($resolvedCandidate -ne $resolvedRoot -and -not $resolvedCandidate.StartsWith($resolvedRootWithSeparator, [System.StringComparison]::OrdinalIgnoreCase)) {
            Write-Response $stream 403 'Forbidden' 'text/plain; charset=utf-8' ([System.Text.Encoding]::UTF8.GetBytes('Forbidden'))
            return
        }

        $bytes = [System.IO.File]::ReadAllBytes($resolvedCandidate)
        if ($method -eq 'HEAD') {
            $bytes = [byte[]]::new(0)
        }
        Write-Response $stream 200 'OK' (Get-ContentType $resolvedCandidate) $bytes
    } finally {
        $client.Close()
    }
}

try {
    while ($true) {
        $handled = $false
        foreach ($listener in $listeners) {
            if ($listener.Pending()) {
                Handle-Client $listener.AcceptTcpClient()
                $handled = $true
            }
        }
        if (-not $handled) {
            Start-Sleep -Milliseconds 25
        }
    }
}
finally {
    foreach ($listener in $listeners) {
        $listener.Stop()
    }
}
