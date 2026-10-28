# kaze benchmark harness for Windows. Usage:
#   pwsh -File bench/run.ps1 [-Out <dir>] [-Capture <claude capture>] [-LaunchRuns 5]
#
# Every number comes from kaze's own KAZE_BENCH mode (see bench/README.md) or
# from reading the kaze process's counters. The harness never sends keyboard or
# mouse input to any window: synthetic keys are injected inside kaze.
# It only stops processes it started, by PID, and every run has a deadline.
# Defaults sit next to the checkout: <parent>\kaze-wt\bench for output and
# <parent>\kaze-wt\captures\claude-7.bin for the capture (where Task 7 left it).
param(
  [string]$Out = (Join-Path (Split-Path (Split-Path $PSScriptRoot -Parent) -Parent) 'kaze-wt\bench'),
  [string]$Capture = (Join-Path (Split-Path (Split-Path $PSScriptRoot -Parent) -Parent) 'kaze-wt\captures\claude-7.bin'),
  [int]$LaunchRuns = 5,
  # A Go temp dir outside %TEMP% helps when Smart App Control blocks test binaries; empty keeps Go's default.
  [string]$GoTmp = $env:GOTMPDIR,
  # Run only these scenarios: launch, keys, flood, claude, idle, full.
  [string[]]$Only = @()
)
$ErrorActionPreference = 'Stop'
$repo = Split-Path $PSScriptRoot -Parent
$env:PATH = "$env:LOCALAPPDATA\Programs\go\bin;$env:PATH"
if ($GoTmp) { $env:GOTMPDIR = $GoTmp }
New-Item -ItemType Directory -Force $Out | Out-Null
# kaze reads its config from %APPDATA%\kaze\config.toml, so each run points APPDATA at one of these.
# default: no config, kaze as shipped (pwsh). quiet: pwsh with history saving off, for runs that
# type a command (the Store pwsh keeps its history under the real profile whatever APPDATA says).
# cmd: cmd.exe, to show launch time without PowerShell's own startup.
$configs = @{
  default = ''
  quiet   = 'shell = ["pwsh", "-NoLogo", "-NoExit", "-Command", "Set-PSReadLineOption -HistorySaveStyle SaveNothing"]'
  cmd     = 'shell = ["cmd.exe"]'
}
foreach ($c in $configs.Keys) {
  $dir = Join-Path $Out "appdata-$c\kaze"
  New-Item -ItemType Directory -Force $dir | Out-Null
  if ($configs[$c]) { Set-Content -Encoding utf8NoBOM (Join-Path $dir 'config.toml') $configs[$c] }
}
$kaze = Join-Path $Out 'kaze-bench.exe'
$flood = Join-Path $Out 'flood.exe'

Add-Type -Namespace KazeBench -Name Win -MemberDefinition @'
[DllImport("user32.dll")] public static extern System.IntPtr GetForegroundWindow();
[DllImport("user32.dll")] public static extern uint GetWindowThreadProcessId(System.IntPtr h, out uint pid);
'@
function Get-ForegroundPid {
  $fpid = [uint32]0
  [void][KazeBench.Win]::GetWindowThreadProcessId([KazeBench.Win]::GetForegroundWindow(), [ref]$fpid)
  return [int]$fpid
}

# Smart App Control may block a fresh binary; a rebuild with a new link-time value gets a new hash.
function Build([string]$pkg, [string]$exe, [string]$extra) {
  Push-Location $repo
  try {
    go build -ldflags "$extra -s -w -X main.r=$(Get-Random)" -o $exe $pkg
    if ($LASTEXITCODE -ne 0) { throw "go build $pkg failed" }
  } finally { Pop-Location }
}

function Build-Flood {
  for ($try = 1; $try -le 8; $try++) {
    Build './bench/flood' $flood ''
    try { $o = & $flood -lines 1 2>&1; if ($LASTEXITCODE -eq 0) { return } } catch {}
    Write-Host "flood.exe blocked (try $try); rebuilding"
  }
  throw 'flood.exe stays blocked by Application Control'
}

function Get-Descendants([int]$id) {
  $kids = @(Get-CimInstance Win32_Process -Filter "ParentProcessId=$id" -ErrorAction SilentlyContinue)
  foreach ($k in $kids) { $k.ProcessId; Get-Descendants $k.ProcessId }
}

function Stop-Tree([int[]]$ids) {
  foreach ($i in $ids) { Stop-Process -Id $i -Force -ErrorAction SilentlyContinue }
}

# Start-Detached starts exe through WMI, so its parent is the WMI host and not this
# shell, and shows it with SW_SHOWNOACTIVATE (4). Windows usually refuses the
# foreground to a process outside the foreground app's tree, but not always (Gio
# asks for it, and Windows allows it after a stretch with no input), so kaze's
# bench mode also drops all real input by default.
function Start-Detached([string]$exe, [hashtable]$vars) {
  $envs = @{}
  foreach ($e in [Environment]::GetEnvironmentVariables().GetEnumerator()) { $envs[$e.Key] = $e.Value }
  foreach ($k in $vars.Keys) { $envs[$k] = $vars[$k] }
  $list = [string[]]@($envs.GetEnumerator() | ForEach-Object { "$($_.Key)=$($_.Value)" })
  $si = New-CimInstance -ClassName Win32_ProcessStartup -ClientOnly -Property @{ EnvironmentVariables = $list; ShowWindow = [uint16]4 }
  $r = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{
    CommandLine = "`"$exe`""; CurrentDirectory = (Split-Path $exe); ProcessStartupInformation = $si }
  if ($r.ReturnValue -ne 0) { return $null }
  try { return Get-Process -Id $r.ProcessId -ErrorAction Stop } catch { return $null }
}

# Run-Kaze starts kaze-bench with KAZE_BENCH_* set, samples its private bytes and CPU
# time at the given seconds, waits for it to close itself, and returns the log.
function Run-Kaze([string]$name, [hashtable]$vars, [int]$seconds, [int[]]$sampleAt = @(), [string]$config = 'quiet') {
  $log = Join-Path $Out "$name.log"
  Remove-Item $log -ErrorAction SilentlyContinue
  # ALLOW_INPUT is forced empty so real input stays dropped even if this shell has it set.
  $all = @{ KAZE_BENCH = '1'; KAZE_BENCH_LOG = $log; KAZE_BENCH_SECONDS = "$seconds"; KAZE_BENCH_ALLOW_INPUT = ''
    APPDATA = (Join-Path $Out "appdata-$config") } + $vars
  $p = $null
  for ($try = 1; $try -le 8 -and -not $p; $try++) {
    $p = Start-Detached $kaze $all
    if (-not $p) { Write-Host "kaze-bench.exe did not start (try $try); rebuilding"; Build './cmd/kaze' $kaze '-H=windowsgui' }
  }
  if (-not $p) { throw 'kaze-bench.exe stays blocked by Application Control' }
  $sw = [Diagnostics.Stopwatch]::StartNew()
  $tree = [Collections.Generic.HashSet[int]]::new()
  $samples = @()
  $fg = $false
  foreach ($s in $sampleAt) {
    while ($sw.Elapsed.TotalSeconds -lt $s -and -not $p.HasExited) {
      Start-Sleep -Milliseconds 200
      if (-not $fg -and (Get-ForegroundPid) -eq $p.Id) { $fg = $true }
    }
    if ($p.HasExited) { break }
    $p.Refresh()
    foreach ($d in @(Get-Descendants $p.Id)) { [void]$tree.Add([int]$d) }
    $samples += [pscustomobject]@{ t = $s; private_mb = [math]::Round($p.PrivateMemorySize64 / 1MB, 1); cpu_ms = $p.TotalProcessorTime.TotalMilliseconds }
  }
  $limit = ($seconds + 30) * 1000
  while (-not $p.HasExited -and $sw.ElapsedMilliseconds -lt $limit) {
    if ($sw.Elapsed.TotalSeconds -lt 5) { foreach ($d in @(Get-Descendants $p.Id)) { [void]$tree.Add([int]$d) } }
    if (-not $fg -and (Get-ForegroundPid) -eq $p.Id) { $fg = $true }
    Start-Sleep -Milliseconds 250
  }
  $killed = $false
  if (-not $p.HasExited) { $killed = $true; Stop-Tree (@($p.Id) + @(Get-Descendants $p.Id)) }
  # Shells or flood left behind by a run that did not close cleanly. A PID can be reused once its
  # process exits, so only stop ones that are still the kind kaze starts and started during this run.
  foreach ($i in $tree) {
    $q = Get-Process -Id $i -ErrorAction SilentlyContinue
    if ($q -and $q.StartTime -ge $p.StartTime -and @('pwsh', 'cmd', 'flood', 'conhost', 'OpenConsole') -contains $q.ProcessName) {
      Stop-Process -Id $i -Force -ErrorAction SilentlyContinue
    }
  }
  $lines = @()
  if (Test-Path $log) { $lines = @(Get-Content $log | Where-Object { $_ } | ForEach-Object { $_ | ConvertFrom-Json }) }
  [pscustomobject]@{
    name = $name; samples = $samples; killed = $killed; took_foreground = $fg
    launch = ($lines | Where-Object event -eq 'launch' | Select-Object -First 1)
    summary = ($lines | Where-Object event -eq 'summary' | Select-Object -Last 1)
    screen = (($lines | Where-Object event -eq 'screen' | Select-Object -Last 1).lines | Where-Object { $_ } | Select-Object -Last 6)
  }
}

function Want([string]$s) { $Only.Count -eq 0 -or $Only -contains $s }

function CpuPct($a, $b) { [math]::Round(100 * ($b.cpu_ms - $a.cpu_ms) / (1000 * ($b.t - $a.t)), 3) }

Write-Host 'building kaze-bench.exe and flood.exe'
Build './cmd/kaze' $kaze '-H=windowsgui'
Build-Flood

$res = [ordered]@{ date = (Get-Date).ToString('yyyy-MM-dd HH:mm'); machine = $env:COMPUTERNAME; cpu = (Get-CimInstance Win32_Processor | Select-Object -First 1).Name }

if (Want 'launch') {
Write-Host 'launch'
$res.launch = @(1..$LaunchRuns | ForEach-Object { Run-Kaze "launch-$_" @{} 5 @() 'default' })
$res.launch_cmd = @(1..3 | ForEach-Object { Run-Kaze "launch-cmd-$_" @{} 5 @() 'cmd' })

}
if (Want 'keys') {
Write-Host 'keys: 200 synthetic keys, 50ms apart'
$res.keys = Run-Kaze 'keys' @{ KAZE_BENCH_KEYS = '200'; KAZE_BENCH_PACE_MS = '50' } 40

}
if (Want 'flood') {
Write-Host 'flood 10000 colored lines, then scroll through history'
# 10100 lines so history (capped at 10000) is full once the screen's rows are subtracted.
$res.flood = Run-Kaze 'flood-scroll' @{ KAZE_BENCH_TYPE = "& '$flood' -lines 10100"; KAZE_BENCH_SCROLL = '10000' } 60

}
if (Want 'claude') {
Write-Host 'claude capture replay'
if (Test-Path $Capture) {
  $res.claude = Run-Kaze 'claude' @{ KAZE_BENCH_TYPE = "& '$flood' -claude '$Capture' -seconds 10" } 20
} else { $res.claude = "capture not found: $Capture" }

}
if (Want 'idle') {
Write-Host 'idle: 1 tab, then 4 tabs (memory and CPU)'
$res.idle1 = Run-Kaze 'idle-1' @{ KAZE_BENCH_TABS = '1' } 25 @(20) 'default'
$res.idle4 = Run-Kaze 'idle-4' @{ KAZE_BENCH_TABS = '4' } 75 @(10, 40, 70) 'default'

}
if (Want 'full') {
Write-Host 'per-tab cost with 10000 lines of history: 1 tab, then 4 tabs'
$res.full1 = Run-Kaze 'full-1' @{ KAZE_BENCH_TABS = '1'; KAZE_BENCH_TYPE = "& '$flood' -lines 10100" } 30 @(25)
$res.full4 = Run-Kaze 'full-4' @{ KAZE_BENCH_TABS = '4'; KAZE_BENCH_TYPE = "& '$flood' -lines 10100" } 30 @(25)

}

# Derived numbers.
$l = @($res.launch | ForEach-Object { $_.launch.launch_ms } | Where-Object { $_ } | Sort-Object)
$lc = @($res.launch_cmd | ForEach-Object { $_.launch.launch_ms } | Where-Object { $_ } | Sort-Object)
$i4 = $res.idle4.samples
$d = [ordered]@{
  launch_ms = $l
  launch_median_ms = if ($l.Count) { $l[[int][math]::Floor(($l.Count - 1) / 2)] } else { $null }
  launch_cmd_ms = $lc
  first_frame_ms = @($res.launch | ForEach-Object { $_.launch.first_frame_ms })
  idle4_cpu_pct_one_core_30s = if ($i4.Count -ge 2) { CpuPct $i4[0] $i4[1] } else { $null }
  idle4_mb = @($i4 | ForEach-Object { $_.private_mb })
  idle4_growth_60s_mb = if ($i4.Count -ge 3) { [math]::Round($i4[2].private_mb - $i4[0].private_mb, 1) } else { $null }
  idle1_mb = @($res.idle1.samples | ForEach-Object { $_.private_mb })
  full1_mb = @($res.full1.samples | ForEach-Object { $_.private_mb })
  full4_mb = @($res.full4.samples | ForEach-Object { $_.private_mb })
}
if ($d.full1_mb.Count -and $d.full4_mb.Count) { $d.per_full_tab_mb = [math]::Round(($d.full4_mb[0] - $d.full1_mb[0]) / 3, 1) }
if ($d.idle1_mb.Count -and $i4.Count) { $d.per_idle_tab_mb = [math]::Round(($i4[0].private_mb - $d.idle1_mb[0]) / 3, 1) }
$res.derived = $d
$res.took_foreground = @($res.Values | ForEach-Object { $_ } | Where-Object { $_.took_foreground } | ForEach-Object { $_.name })

$json = Join-Path $Out 'results.json'
$res | ConvertTo-Json -Depth 8 | Set-Content -Encoding utf8 $json
Write-Host "results: $json"
$d | ConvertTo-Json -Depth 4
