# Downloads the pinned Paradee model plus ONNX Runtime 1.20.0.
# Kokoro is optional: pass -All (or set MDVIEWER_NEURAL_KOKORO=1). It is hidden
# by default because this class of CPU renders it far below real time.
param([switch]$All)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
$Dest = Join-Path $Root "resources\neural"
$Piper = Join-Path $Root "resources\piper\engine\windows\piper"
New-Item -ItemType Directory -Force -Path $Dest | Out-Null

if ($All -or $env:MDVIEWER_NEURAL_KOKORO -eq "1") {
    $Kokoro = $true
} else {
    $Kokoro = $false
}

function Get-Sha256($path) {
    $stream = [System.IO.File]::OpenRead($path)
    try {
        $sha = [System.Security.Cryptography.SHA256]::Create()
        return ([BitConverter]::ToString($sha.ComputeHash($stream)) -replace '-', '').ToLower()
    } finally {
        $stream.Dispose()
    }
}

function Get-Pinned($uri, $out, $sha) {
    if (Test-Path $out) {
        $existing = Get-Sha256 $out
        if ($existing -eq $sha) { Write-Host "present: $(Split-Path -Leaf $out)"; return }
        Remove-Item $out -Force
    }
    for ($i = 1; $i -le 3; $i++) {
        try { Invoke-WebRequest -Uri $uri -OutFile $out; break }
        catch { if ($i -eq 3) { throw }; Start-Sleep -Seconds 2 }
    }
    $got = Get-Sha256 $out
    if ($got -ne $sha) {
        Remove-Item $out -Force
        throw "checksum mismatch for $(Split-Path -Leaf $out): $got"
    }
}

Get-Pinned "https://huggingface.co/sahilmahendrakar/Paradee-8M-v1.2/resolve/main/onnx/paradee_int8.onnx" `
    (Join-Path $Dest "paradee_int8.onnx") `
    "a6989796692ac173597e15d405c971989202d5c73cfad65f22e90bd7b6c97da1"

if ($Kokoro) {
    Get-Pinned "https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0/kokoro-v1.0.int8.onnx" `
        (Join-Path $Dest "kokoro-v1.0.int8.onnx") `
        "6e742170d309016e5891a994e1ce1559c702a2ccd0075e67ef7157974f6406cb"

    Get-Pinned "https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0/voices-v1.0.bin" `
        (Join-Path $Dest "voices-v1.0.bin") `
        "bca610b8308e8d99f32e6fe4197e7ec01679264efed0cac9140fe9c29f1fbf7d"
}

$OrtDll = Join-Path $Dest "onnxruntime.dll"
if (-not (Test-Path $OrtDll)) {
    $zip = Join-Path $env:TEMP "onnxruntime-1.20.0.zip"
    Get-Pinned "https://github.com/microsoft/onnxruntime/releases/download/v1.20.0/onnxruntime-win-x64-1.20.0.zip" `
        $zip "b372de85cedd9387a0d4386b982265e8420e5bcc2f29394317e76525b832942e"
    $tmp = Join-Path $env:TEMP "onnxruntime-1.20.0"
    if (Test-Path $tmp) { Remove-Item $tmp -Recurse -Force }
    Expand-Archive -Path $zip -DestinationPath $tmp -Force
    Copy-Item (Join-Path $tmp "onnxruntime-win-x64-1.20.0\lib\onnxruntime.dll") $OrtDll
    Remove-Item $zip -Recurse -Force
    Remove-Item $tmp -Recurse -Force
}

$espeak = Join-Path $Piper "espeak-ng.dll"
if (-not (Test-Path $espeak)) {
    throw "espeak-ng.dll missing; run scripts/fetch-piper.ps1 first"
}
Copy-Item $espeak (Join-Path $Dest "espeak-ng.dll") -Force

foreach ($cfg in @("paradee-config.json", "kokoro-config.json")) {
    if (-not (Test-Path (Join-Path $Dest $cfg))) {
        throw "$cfg is missing from resources\neural (it is committed to the repo)"
    }
}

Write-Host "Neural assets ready in resources\neural"
