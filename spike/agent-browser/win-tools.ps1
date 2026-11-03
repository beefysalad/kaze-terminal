# Windows helper for probe.mjs. Reads one JSON request per line on stdin, answers one JSON line on stdout.
# Requests: {"id":1,"cmd":"launch","exe":"...","args":[...],"show":4}  start a process with STARTUPINFO.wShowWindow
#           {"id":2,"cmd":"tree","pid":123}                            the process and its descendants, with memory
#           {"id":3,"cmd":"windows","pids":[...]}                       top-level windows owned by these processes
#           {"id":4,"cmd":"leftovers","marker":"...","kill":false}       msedge/chrome processes whose command line has marker
# Read-only towards other programs: it never activates windows, never sends input, and only kills
# browser processes whose command line contains this run's private profile directory.
$ErrorActionPreference = 'Stop'

Add-Type -Language CSharp -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text;

public static class KazeWinTools {
    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    struct STARTUPINFO {
        public int cb; public string lpReserved; public string lpDesktop; public string lpTitle;
        public int dwX; public int dwY; public int dwXSize; public int dwYSize; public int dwXCountChars; public int dwYCountChars;
        public int dwFillAttribute; public int dwFlags; public short wShowWindow; public short cbReserved2;
        public IntPtr lpReserved2; public IntPtr hStdInput; public IntPtr hStdOutput; public IntPtr hStdError;
    }
    [StructLayout(LayoutKind.Sequential)]
    struct PROCESS_INFORMATION { public IntPtr hProcess; public IntPtr hThread; public int dwProcessId; public int dwThreadId; }
    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    struct PROCESSENTRY32 {
        public uint dwSize; public uint cntUsage; public uint th32ProcessID; public IntPtr th32DefaultHeapID;
        public uint th32ModuleID; public uint cntThreads; public uint th32ParentProcessID; public int pcPriClassBase;
        public uint dwFlags; [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 260)] public string szExeFile;
    }
    [StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left, Top, Right, Bottom; }
    delegate bool EnumProc(IntPtr hwnd, IntPtr lParam);

    [DllImport("kernel32.dll", SetLastError = true, CharSet = CharSet.Unicode)]
    static extern bool CreateProcessW(string app, StringBuilder cmd, IntPtr pa, IntPtr ta, bool inherit, uint flags, IntPtr env, string cwd, ref STARTUPINFO si, out PROCESS_INFORMATION pi);
    [DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr handle);
    [DllImport("kernel32.dll", SetLastError = true)] static extern IntPtr CreateToolhelp32Snapshot(uint flags, uint pid);
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode)] static extern bool Process32FirstW(IntPtr snap, ref PROCESSENTRY32 entry);
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode)] static extern bool Process32NextW(IntPtr snap, ref PROCESSENTRY32 entry);
    [DllImport("user32.dll")] static extern bool EnumWindows(EnumProc proc, IntPtr lParam);
    [DllImport("user32.dll")] static extern uint GetWindowThreadProcessId(IntPtr hwnd, out uint pid);
    [DllImport("user32.dll")] static extern bool IsWindowVisible(IntPtr hwnd);
    [DllImport("user32.dll")] static extern bool IsIconic(IntPtr hwnd);
    [DllImport("user32.dll")] static extern bool GetWindowRect(IntPtr hwnd, out RECT rect);
    [DllImport("user32.dll")] static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] static extern int GetClassName(IntPtr hwnd, StringBuilder name, int max);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] static extern int GetWindowText(IntPtr hwnd, StringBuilder text, int max);
    [DllImport("user32.dll")] static extern int GetWindowLong(IntPtr hwnd, int index);

    static string Quote(string arg) {
        if (arg.Length > 0 && arg.IndexOfAny(new[] { ' ', '\t', '"' }) < 0) return arg;
        var b = new StringBuilder("\"");
        int slashes = 0;
        foreach (char c in arg) {
            if (c == '\\') { slashes++; continue; }
            if (c == '"') { b.Append('\\', slashes * 2 + 1).Append('"'); slashes = 0; continue; }
            b.Append('\\', slashes).Append(c); slashes = 0;
        }
        b.Append('\\', slashes * 2).Append('"');
        return b.ToString();
    }

    // STARTF_USESHOWWINDOW with the given show command (4 = SW_SHOWNOACTIVATE, 7 = SW_SHOWMINNOACTIVE).
    public static int Launch(string exe, string[] args, short show) {
        var cmd = new StringBuilder(Quote(exe));
        foreach (var a in args) cmd.Append(' ').Append(Quote(a));
        var si = new STARTUPINFO { cb = Marshal.SizeOf(typeof(STARTUPINFO)), dwFlags = 0x1, wShowWindow = show };
        PROCESS_INFORMATION pi;
        if (!CreateProcessW(exe, cmd, IntPtr.Zero, IntPtr.Zero, false, 0, IntPtr.Zero, null, ref si, out pi))
            throw new Exception("CreateProcess failed: " + Marshal.GetLastWin32Error());
        CloseHandle(pi.hThread); CloseHandle(pi.hProcess);
        return pi.dwProcessId;
    }

    public static Dictionary<uint, uint> ParentMap() {
        var map = new Dictionary<uint, uint>();
        IntPtr snap = CreateToolhelp32Snapshot(0x2, 0);
        if (snap == IntPtr.Zero || snap == new IntPtr(-1)) return map;
        try {
            var entry = new PROCESSENTRY32 { dwSize = (uint)Marshal.SizeOf(typeof(PROCESSENTRY32)) };
            if (Process32FirstW(snap, ref entry)) {
                do { map[entry.th32ProcessID] = entry.th32ParentProcessID; } while (Process32NextW(snap, ref entry));
            }
        } finally { CloseHandle(snap); }
        return map;
    }

    public static List<object> Tree(uint root) {
        var parents = ParentMap();
        var pids = new List<uint>();
        if (parents.ContainsKey(root)) pids.Add(root);
        for (int i = 0; i < pids.Count && pids.Count < 200; i++)
            foreach (var kv in parents) if (kv.Value == pids[i] && kv.Key != pids[i] && !pids.Contains(kv.Key)) pids.Add(kv.Key);
        var rows = new List<object>();
        foreach (var pid in pids) {
            try {
                using (var p = Process.GetProcessById((int)pid)) {
                    rows.Add(new Dictionary<string, object> {
                        { "pid", pid }, { "parent", parents[pid] }, { "name", p.ProcessName },
                        { "workingSetBytes", p.WorkingSet64 }, { "privateBytes", p.PrivateMemorySize64 },
                        { "cpuMs", (long)p.TotalProcessorTime.TotalMilliseconds }
                    });
                }
            } catch { }
        }
        return rows;
    }

    public static List<object> Windows(uint[] pidList) {
        var wanted = new HashSet<uint>(pidList);
        var rows = new List<object>();
        IntPtr foreground = GetForegroundWindow();
        int visibleIndex = 0, foregroundIndex = -1;
        EnumWindows((hwnd, _) => {
            bool visible = IsWindowVisible(hwnd);
            if (hwnd == foreground) foregroundIndex = visibleIndex;
            uint pid; GetWindowThreadProcessId(hwnd, out pid);
            var title = new StringBuilder(512);
            if (wanted.Contains(pid)) GetWindowText(hwnd, title, title.Capacity);
            // Visible windows, plus hidden ones with a title (a browser window that exists but was never shown).
            if (wanted.Contains(pid) && (visible || title.Length > 0)) {
                RECT r; GetWindowRect(hwnd, out r);
                var cls = new StringBuilder(256); GetClassName(hwnd, cls, cls.Capacity);
                int exStyle = GetWindowLong(hwnd, -20);
                rows.Add(new Dictionary<string, object> {
                    { "hwnd", hwnd.ToInt64() }, { "pid", pid }, { "visible", visible }, { "class", cls.ToString() }, { "title", title.ToString() },
                    { "minimized", IsIconic(hwnd) }, { "foreground", hwnd == foreground }, { "topmost", (exStyle & 0x8) != 0 },
                    { "zIndex", visibleIndex }, { "rect", new int[] { r.Left, r.Top, r.Right, r.Bottom } }
                });
            }
            if (visible) visibleIndex++;
            return true;
        }, IntPtr.Zero);
        foreach (Dictionary<string, object> row in rows) row["aboveForegroundWindow"] = (bool)row["visible"] && foregroundIndex >= 0 && (int)row["zIndex"] < foregroundIndex;
        return rows;
    }
}
'@

$stdin = [Console]::In
while ($null -ne ($line = $stdin.ReadLine())) {
  if (-not $line.Trim()) { continue }
  $request = $null
  try {
    $request = $line | ConvertFrom-Json
    $result = switch ($request.cmd) {
      'launch' { @{ pid = [KazeWinTools]::Launch($request.exe, [string[]]@($request.args), [int16]$request.show) } }
      'tree' { , @([KazeWinTools]::Tree([uint32]$request.pid)) }
      'windows' { , @([KazeWinTools]::Windows([uint32[]]@($request.pids))) }
      'leftovers' {
        $found = @(Get-CimInstance Win32_Process -Filter "Name='msedge.exe' OR Name='chrome.exe'" |
          Where-Object { $_.CommandLine -and $_.CommandLine.Contains([string]$request.marker) } |
          ForEach-Object { @{ pid = $_.ProcessId; parent = $_.ParentProcessId; name = $_.Name } })
        if ($request.kill) { foreach ($p in $found) { try { Stop-Process -Id $p.pid -Force -ErrorAction Stop } catch {} } }
        , $found
      }
      default { throw "unknown command: $($request.cmd)" }
    }
    [Console]::Out.WriteLine((@{ id = $request.id; ok = $true; result = $result } | ConvertTo-Json -Compress -Depth 8))
  } catch {
    [Console]::Out.WriteLine((@{ id = $request.id; ok = $false; error = "$_" } | ConvertTo-Json -Compress -Depth 4))
  }
  [Console]::Out.Flush()
}
