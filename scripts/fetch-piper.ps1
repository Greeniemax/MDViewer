# Downloads the pinned Piper engine (Windows) and default voices into resources/.
$ErrorActionPreference = "Stop"
$PiperVersion = "2023.11.14-2"
$Root   = Split-Path -Parent $PSScriptRoot
$Eng    = Join-Path $Root "resources\piper\engine"
$Voices = Join-Path $Root "resources\piper\voices"
$Rel    = "https://github.com/rhasspy/piper/releases/download/$PiperVersion"
$Hf     = "https://huggingface.co/rhasspy/piper-voices/resolve/main"
New-Item -ItemType Directory -Force -Path $Eng, $Voices | Out-Null

function Get-FileWithRetry($uri, $out) {
    for ($i = 1; $i -le 3; $i++) {
        try { Invoke-WebRequest -Uri $uri -OutFile $out; return }
        catch { if ($i -eq 3) { throw }; Start-Sleep -Seconds 2 }
    }
}

function Fetch-Voice($key, $sub) {
    $onnx     = Join-Path $Voices "$key.onnx"
    $onnxJson = Join-Path $Voices "$key.onnx.json"
    if ((Test-Path $onnx) -and (Test-Path $onnxJson)) { Write-Host "voice $key present"; return }
    Get-FileWithRetry "$Hf/$sub/$key.onnx?download=true"      $onnx
    Get-FileWithRetry "$Hf/$sub/$key.onnx.json?download=true" $onnxJson
}

$winDir = Join-Path $Eng "windows"
if (-not (Test-Path (Join-Path $winDir "piper\piper.exe"))) {
    New-Item -ItemType Directory -Force -Path $winDir | Out-Null
    $zip = Join-Path $env:TEMP "piper_win.zip"
    Get-FileWithRetry "$Rel/piper_windows_amd64.zip" $zip
    Expand-Archive -Path $zip -DestinationPath $winDir -Force   # creates piper\ subdir
    Remove-Item $zip
} else { Write-Host "engine windows present" }

Fetch-Voice "en_US-amy-medium"  "en/en_US/amy/medium"
Fetch-Voice "en_US-ryan-medium" "en/en_US/ryan/medium"

Write-Host "Piper assets ready in resources\piper\"
