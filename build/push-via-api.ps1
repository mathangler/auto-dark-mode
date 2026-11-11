param(
    [string]$Repo = 'mathangler/auto-dark-mode',
    [string]$Branch = 'main'
)
$ErrorActionPreference = 'Stop'
$app = Split-Path -Parent $PSScriptRoot
Push-Location $app
try {
    $token = (gh auth token --user mathangler) 2>$null
    if (-not $token) { throw 'no mathangler token from gh' }
    $headers = @{
        Authorization          = "Bearer $token"
        'User-Agent'           = 'AutoDarkMode'
        Accept                 = 'application/vnd.github+json'
        'X-GitHub-Api-Version' = '2022-11-28'
    }
    $api = "https://api.github.com/repos/$Repo"

    $msg = ((git log -1 --pretty=%B) -join "`n").Trim()
    if (-not $msg) { throw 'empty commit message' }

    # Parent = current remote head, so history stays linear even though the
    # previous commit was created through this API (and is not in the clone).
    $ref = Invoke-RestMethod "$api/git/ref/heads/$Branch" -Headers $headers
    $parent = $ref.object.sha
    Write-Host "parent  $parent"

    # Build a complete tree from the local index. This avoids any base_tree
    # mismatch: the pushed tree is exactly what the local HEAD contains.
    $ls = git ls-tree -r HEAD
    $entries = New-Object System.Collections.Generic.List[object]
    foreach ($line in $ls) {
        if (-not ($line -match '^(\d+)\s+(\w+)\s+([0-9a-f]+)\t(.+)$')) { continue }
        $mode = $Matches[1]
        $type = $Matches[2]
        $path = ($Matches[4] -replace '\\', '/')
        if ($type -ne 'blob') { Write-Host "skip    $path ($type)"; continue }
        $full = Join-Path $app $path
        if (-not (Test-Path $full)) { throw "missing file: $path" }
        $b64 = [Convert]::ToBase64String([IO.File]::ReadAllBytes($full))
        $body = @{ content = $b64; encoding = 'base64' } | ConvertTo-Json -Compress
        $blob = Invoke-RestMethod "$api/git/blobs" -Method Post -Headers $headers -Body $body
        $entries.Add(@{ path = $path; mode = $mode; type = 'blob'; sha = $blob.sha })
    }
    Write-Host "files   $($entries.Count)"

    $treeBody = @{ tree = $entries } | ConvertTo-Json -Depth 6 -Compress
    $tree = Invoke-RestMethod "$api/git/trees" -Method Post -Headers $headers -Body $treeBody
    Write-Host "tree    $($tree.sha)"

    $commitBody = @{ message = $msg; tree = $tree.sha; parents = @($parent) } | ConvertTo-Json -Depth 6 -Compress
    $commit = Invoke-RestMethod "$api/git/commits" -Method Post -Headers $headers -Body $commitBody
    Write-Host "commit  $($commit.sha)"

    $refBody = @{ sha = $commit.sha; force = $false } | ConvertTo-Json -Compress
    Invoke-RestMethod "$api/git/refs/heads/$Branch" -Method Patch -Headers $headers -Body $refBody | Out-Null
    Write-Host "pushed  $Branch -> $($commit.sha)"
    Write-Output $commit.sha
} finally {
    Pop-Location
}
