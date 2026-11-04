# Windows counterpart of focus-monitor.m: a passive foreground-window observer.
# It only listens to WinEvents (out of context) and reads process/window facts.
# It never calls SetForegroundWindow, never sends input and never touches other windows.
# Output: one JSON object per line on stdout.
#   {"kind":"baseline"|"foreground"|"show","at_ms":...,"pid":...,"exe":"...","hwnd":...,"class":"...","ancestors":[...]}
# "show" lines are top-level windows becoming visible (EVENT_OBJECT_SHOW); the probe keeps only its own.
param([switch]$ExitOnStdinClose)
$ErrorActionPreference = 'Stop'

Add-Type -Language CSharp -TypeDefinition @'
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;

public static class KazeFocusMonitor {
    delegate void WinEventProc(IntPtr hook, uint evt, IntPtr hwnd, int idObject, int idChild, uint thread, uint time);

    [StructLayout(LayoutKind.Sequential)]
    struct MSG { public IntPtr hwnd; public uint message; public UIntPtr wParam; public IntPtr lParam; public uint time; public int x; public int y; public uint lPrivate; }

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    struct PROCESSENTRY32 {
        public uint dwSize; public uint cntUsage; public uint th32ProcessID; public IntPtr th32DefaultHeapID;
        public uint th32ModuleID; public uint cntThreads; public uint th32ParentProcessID; public int pcPriClassBase;
        public uint dwFlags; [MarshalAs(UnmanagedType.ByValTStr, SizeConst = 260)] public string szExeFile;
    }

    [DllImport("user32.dll")] static extern IntPtr SetWinEventHook(uint min, uint max, IntPtr module, WinEventProc proc, uint pid, uint tid, uint flags);
    [DllImport("user32.dll")] static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")] static extern uint GetWindowThreadProcessId(IntPtr hwnd, out uint pid);
    [DllImport("user32.dll")] static extern IntPtr GetAncestor(IntPtr hwnd, uint flags);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)] static extern int GetClassName(IntPtr hwnd, StringBuilder name, int max);
    [DllImport("user32.dll")] static extern int GetMessage(out MSG msg, IntPtr hwnd, uint min, uint max);
    [DllImport("user32.dll")] static extern bool TranslateMessage(ref MSG msg);
    [DllImport("user32.dll")] static extern IntPtr DispatchMessage(ref MSG msg);
    [DllImport("kernel32.dll", SetLastError = true)] static extern IntPtr CreateToolhelp32Snapshot(uint flags, uint pid);
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode)] static extern bool Process32FirstW(IntPtr snap, ref PROCESSENTRY32 entry);
    [DllImport("kernel32.dll", CharSet = CharSet.Unicode)] static extern bool Process32NextW(IntPtr snap, ref PROCESSENTRY32 entry);
    [DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr handle);

    const uint EVENT_SYSTEM_FOREGROUND = 0x0003;
    const uint EVENT_OBJECT_SHOW = 0x8002;
    const uint WINEVENT_OUTOFCONTEXT = 0x0000;
    const uint WINEVENT_SKIPOWNPROCESS = 0x0002;
    const uint GA_ROOT = 2;

    static WinEventProc keepAlive; // the delegate must outlive the hooks
    static readonly object gate = new object();

    static Dictionary<uint, uint> ParentMap() {
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

    static string Escape(string s) {
        var b = new StringBuilder();
        foreach (char c in s ?? "") {
            if (c == '"' || c == '\\') { b.Append('\\').Append(c); }
            else if (c < 0x20) { b.AppendFormat("\\u{0:x4}", (int)c); }
            else b.Append(c);
        }
        return b.ToString();
    }

    static void Record(string kind, IntPtr hwnd) {
        long at = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds();
        uint pid = 0;
        if (hwnd != IntPtr.Zero) GetWindowThreadProcessId(hwnd, out pid);
        string exe = "unknown";
        try { if (pid != 0) exe = Process.GetProcessById((int)pid).ProcessName; } catch { }
        var cls = new StringBuilder(256);
        if (hwnd != IntPtr.Zero) GetClassName(hwnd, cls, cls.Capacity);
        var ancestors = new List<uint>();
        var parents = ParentMap();
        uint cursor = pid;
        for (int depth = 0; depth < 8 && parents.ContainsKey(cursor); depth++) {
            uint parent = parents[cursor];
            if (parent == 0 || parent == cursor || ancestors.Contains(parent)) break;
            ancestors.Add(parent);
            cursor = parent;
        }
        string line = "{\"kind\":\"" + kind + "\",\"at_ms\":" + at + ",\"pid\":" + pid + ",\"exe\":\"" + Escape(exe) +
            "\",\"hwnd\":" + hwnd.ToInt64() + ",\"class\":\"" + Escape(cls.ToString()) + "\",\"ancestors\":[" + string.Join(",", ancestors) + "]}";
        lock (gate) { Console.Out.WriteLine(line); Console.Out.Flush(); }
    }

    static void OnEvent(IntPtr hook, uint evt, IntPtr hwnd, int idObject, int idChild, uint thread, uint time) {
        try {
            if (evt == EVENT_SYSTEM_FOREGROUND) { Record("foreground", hwnd); return; }
            // Only top-level windows (not controls, carets or cursors).
            if (evt == EVENT_OBJECT_SHOW && idObject == 0 && idChild == 0 && hwnd != IntPtr.Zero && GetAncestor(hwnd, GA_ROOT) == hwnd) Record("show", hwnd);
        } catch { }
    }

    public static void Run(bool exitOnStdinClose) {
        if (exitOnStdinClose) {
            // The parent probe owns our stdin; when it goes away, so do we (no orphaned observer).
            var watch = new Thread(() => { try { while (Console.In.ReadLine() != null) { } } catch { } Environment.Exit(0); });
            watch.IsBackground = true;
            watch.Start();
        }
        keepAlive = OnEvent;
        Record("baseline", GetForegroundWindow());
        uint flags = WINEVENT_OUTOFCONTEXT | WINEVENT_SKIPOWNPROCESS;
        if (SetWinEventHook(EVENT_SYSTEM_FOREGROUND, EVENT_SYSTEM_FOREGROUND, IntPtr.Zero, keepAlive, 0, 0, flags) == IntPtr.Zero) throw new Exception("SetWinEventHook(foreground) failed");
        if (SetWinEventHook(EVENT_OBJECT_SHOW, EVENT_OBJECT_SHOW, IntPtr.Zero, keepAlive, 0, 0, flags) == IntPtr.Zero) throw new Exception("SetWinEventHook(show) failed");
        MSG msg;
        while (GetMessage(out msg, IntPtr.Zero, 0, 0) > 0) { TranslateMessage(ref msg); DispatchMessage(ref msg); }
    }
}
'@

[KazeFocusMonitor]::Run($ExitOnStdinClose.IsPresent)
