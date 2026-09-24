# Run the Nifty 500 Ichimoku scanner
$envFile = Join-Path $PSScriptRoot ".env"

Get-Content $envFile | ForEach-Object {
    $line = $_.Trim()
    if ($line -eq "" -or $line.StartsWith("#")) { return }
    $idx = $line.IndexOf("=")
    if ($idx -lt 0) { return }
    $key = $line.Substring(0, $idx).Trim()
    $val = $line.Substring($idx + 1).Trim()
    if ($val -match '^(.*?)\s+#') { $val = $matches[1].Trim() }
    [System.Environment]::SetEnvironmentVariable($key, $val, "Process")
}

$gccPath = "C:\msys64\mingw64\bin"
if (Test-Path $gccPath) { $env:PATH = "$gccPath;$env:PATH" }

go run ./cmd/scanner @args
