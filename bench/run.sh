#!/bin/sh
# kaze benchmark harness for macOS (and other Unix). Usage:
#   sh bench/run.sh [out-dir] [claude-capture]
#
# The same scenarios as bench/run.ps1. Every timing comes from kaze's own
# KAZE_BENCH mode (bench/README.md); memory and CPU are read from the kaze
# process. Nothing sends keyboard or mouse input to any window, every run has a
# deadline, and only processes this script started are stopped.
#
# Safety on the Mac: a bare binary can't be launched without activating it
# (`open -g` needs an .app bundle), so the kaze window may take focus. kaze's
# bench mode drops all real keyboard, mouse and paste input by default, so typing
# into it reaches no shell, and this script unsets KAZE_BENCH_ALLOW_INPUT.
set -eu
repo=$(cd "$(dirname "$0")/.." && pwd)
out=${1:-"$repo/bench/out"}
capture=${2:-"$repo/bench/claude.bin"}
kaze="$out/kaze-bench"
flood="$out/flood"
(cd "$repo" && go build -o "$kaze" ./cmd/kaze && go build -o "$flood" ./bench/flood)

# kaze reads <config dir>/kaze/config.toml: $HOME/Library/Application Support on
# macOS, $XDG_CONFIG_HOME elsewhere. Runs point both at $out, with a shell that
# loads no rc files and saves no history, so bench commands stay out of yours.
benchhome="$out/home"
shell='shell = ["/bin/sh", "-c", "if command -v zsh >/dev/null; then HISTFILE=/dev/null exec zsh -f; else HISTFILE=/dev/null exec bash --noprofile --norc; fi"]'
for d in "$benchhome/Library/Application Support/kaze" "$out/config/kaze"; do
  mkdir -p "$d"
  printf '%s\n' "$shell" > "$d/config.toml"
done

# private memory in MB: phys_footprint where footprint(1) exists, else RSS.
mem_mb() {
  if command -v footprint >/dev/null 2>&1; then
    footprint -p "$1" 2>/dev/null | awk '/phys_footprint:/ { v=$2; u=$3; if (u=="KB") v/=1024; if (u=="GB") v*=1024; print v; exit }'
  else
    ps -o rss= -p "$1" | awk '{ printf "%.1f\n", $1/1024 }'
  fi
}
# CPU time in ms from ps's [[dd-]hh:]mm:ss.cc.
cpu_ms() {
  ps -o time= -p "$1" | awk '{ n=split($1, a, ":"); s=0; for (i=1; i<=n; i++) s=s*60+a[i]; printf "%d\n", s*1000 }'
}

# run NAME SECONDS [SAMPLE_SECONDS...] with KAZE_BENCH_* already exported by the caller.
run() {
  name=$1; secs=$2; shift 2
  log="$out/$name.log"; rm -f "$log"
  env -u KAZE_BENCH_ALLOW_INPUT KAZE_BENCH=1 KAZE_BENCH_LOG="$log" KAZE_BENCH_SECONDS="$secs" \
    HOME="$benchhome" XDG_CONFIG_HOME="$out/config" HISTFILE=/dev/null "$kaze" &
  pid=$!
  t=0
  for s in "$@"; do
    while [ $t -lt "$s" ] && kill -0 $pid 2>/dev/null; do sleep 1; t=$((t+1)); done
    kill -0 $pid 2>/dev/null || break
    echo "$name t=$s private_mb=$(mem_mb $pid) cpu_ms=$(cpu_ms $pid)" | tee -a "$out/samples.txt"
  done
  limit=$((secs+30))
  while [ $t -lt $limit ] && kill -0 $pid 2>/dev/null; do sleep 1; t=$((t+1)); done
  if kill -0 $pid 2>/dev/null; then
    pkill -P $pid 2>/dev/null || true
    kill $pid 2>/dev/null || true
    echo "$name: killed after ${limit}s" | tee -a "$out/samples.txt"
  fi
  wait $pid 2>/dev/null || true
  echo "$name: $(tail -n 1 "$log" 2>/dev/null)"
}

: > "$out/samples.txt"
for i in 1 2 3 4 5; do run "launch-$i" 4; done
# Each scenario runs in a subshell: sh keeps assignments made for a function call.
(export KAZE_BENCH_KEYS=200 KAZE_BENCH_PACE_MS=50; run keys 40)
(export KAZE_BENCH_TYPE="'$flood' -lines 10000" KAZE_BENCH_SCROLL=10000; run flood-scroll 60)
if [ -f "$capture" ]; then
  (export KAZE_BENCH_TYPE="'$flood' -claude '$capture' -seconds 10"; run claude 20)
else
  echo "claude: capture not found: $capture"
fi
(export KAZE_BENCH_TABS=1; run idle-1 25 20)
(export KAZE_BENCH_TABS=4; run idle-4 75 10 40 70)
(export KAZE_BENCH_TABS=1 KAZE_BENCH_TYPE="'$flood' -lines 10000"; run full-1 30 25)
(export KAZE_BENCH_TABS=4 KAZE_BENCH_TYPE="'$flood' -lines 10000"; run full-4 30 25)
echo "logs and samples: $out"
