param(
  [ValidatePattern('^[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?$')]
  [string]$Version = "1.0.0",
  [string]$OutputDir = "dist",
  [switch]$Offline
)
$ErrorActionPreference = "Stop"
$root = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot "..")).Path
$dist = [IO.Path]::GetFullPath((Join-Path $root $OutputDir))
$rootPrefix = $root.TrimEnd('\') + '\'
if (!$dist.StartsWith($rootPrefix, [StringComparison]::OrdinalIgnoreCase)) {
  throw 'OutputDir must be a child of the project directory.'
}
# Fresh staging: never recursively delete a caller-derived path.
$stage = Join-Path $dist ('stage-' + [Guid]::NewGuid().ToString('N'))
$zip = Join-Path $dist ("cuemender-windows-x64-{0}.zip" -f $Version)
$go = Join-Path $root '.tools\go1.27.3\go\bin\go.exe'
if (!(Test-Path -LiteralPath $go)) { $go = (Get-Command go -ErrorAction Stop).Source }
if ((& $go version) -notmatch '\bgo1\.27\.3\b') { throw 'Go 1.27.3 is required (see go.mod).' }
$env:CGO_ENABLED = '0'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:GOTOOLCHAIN = 'local'
if (!$env:GOPATH -and (Test-Path -LiteralPath (Join-Path $root '.tools\gopath'))) {
  $env:GOPATH = Join-Path $root '.tools\gopath'
}
if ($Offline) { $env:GOPROXY = 'off' }
foreach ($cacheName in @('gocache','gotmp')) {
  New-Item -ItemType Directory -Force -Path (Join-Path $root ".tools\$cacheName") | Out-Null
}
if (!$env:GOCACHE) { $env:GOCACHE = Join-Path $root '.tools\gocache' }
if (!$env:GOTMPDIR) { $env:GOTMPDIR = Join-Path $root '.tools\gotmp' }
Push-Location -LiteralPath $root
try {
  $commit = 'unknown'
  if (Get-Command git -ErrorAction SilentlyContinue) {
    $head = & git rev-parse --short=12 HEAD 2>$null
    if ($LASTEXITCODE -eq 0) {
      $commit = $head
      $changes = & git status --porcelain --untracked-files=normal
      if ($LASTEXITCODE -ne 0) { throw 'Could not check source status.' }
      if ($changes) { $commit += '-dirty' }
    }
  }
  $buildTime = [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
  New-Item -ItemType Directory -Force -Path $stage | Out-Null
  $ldflags = "-s -w -X github.com/desanv01/cuemender/internal/app.Version=$Version -X github.com/desanv01/cuemender/internal/app.Commit=$commit -X github.com/desanv01/cuemender/internal/app.BuildDate=$buildTime"
  & $go build -trimpath -buildvcs=false -ldflags $ldflags -o (Join-Path $dist 'cuemender.exe') ./cmd/app
  if ($LASTEXITCODE -ne 0) { throw "go build failed with exit $LASTEXITCODE" }
  Copy-Item -LiteralPath (Join-Path $dist 'cuemender.exe') -Destination $stage
  Copy-Item -LiteralPath (Join-Path $root 'LICENSE') -Destination $stage
  foreach ($name in @('README-QUICKSTART.md','THIRD_PARTY_NOTICES.md','GOOBS_PATCH_ATTRIBUTION.md')) {
    Copy-Item -LiteralPath (Join-Path $root "release\$name") -Destination $stage
  }
  Copy-Item -LiteralPath (Join-Path $root 'release\licenses') -Destination $stage -Recurse
  Copy-Item -LiteralPath (Join-Path $root 'README.md') -Destination $stage
  $manifest = @(Get-ChildItem -LiteralPath $stage -Recurse -File | ForEach-Object {
    [pscustomobject]@{
      path = $_.FullName.Substring($stage.Length + 1).Replace('\','/')
      size = $_.Length
      sha256 = (Get-FileHash -Algorithm SHA256 -LiteralPath $_.FullName).Hash
    }
  } | Sort-Object path)
  $manifest | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $dist 'p06-manifest.json') -Encoding UTF8
  $metadata = [pscustomobject]@{version=$Version; commit=$commit; build_time_utc=$buildTime; files=$manifest}
  $metadata | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $stage 'BUILD-MANIFEST.json') -Encoding UTF8
  Compress-Archive -Path (Join-Path $stage '*') -DestinationPath $zip -CompressionLevel Optimal -Force
  $exeHash = (Get-FileHash -Algorithm SHA256 -LiteralPath (Join-Path $dist 'cuemender.exe')).Hash
  $zipHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $zip).Hash
  @("$exeHash  cuemender.exe", "$zipHash  $([IO.Path]::GetFileName($zip))") |
    Set-Content -LiteralPath (Join-Path $dist 'SHA256SUMS.txt') -Encoding ASCII
  [pscustomobject]@{version=$Version; commit=$commit; build_time_utc=$buildTime; executable_sha256=$exeHash; zip=$zip; zip_sha256=$zipHash; files=($manifest.Count+1)} | ConvertTo-Json
} finally {
  Pop-Location
}
