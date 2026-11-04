package ui

import (
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gioui.org/gpu/headless"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/beefysalad/kaze-terminal/internal/battery"
	"github.com/beefysalad/kaze-terminal/internal/theme"
)

// batteryStrip renders the busy status bar with st at the bottom of a w x h
// strip at scale px per dp, headless (no native window, no global input).
func batteryStrip(t *testing.T, key string, st battery.Status, w, h int, scale float32) *image.RGBA {
	t.Helper()
	win := busyWindow(t)
	th, ok := theme.ByKey(key)
	if !ok {
		t.Fatalf("no theme %q", key)
	}
	win.applyTheme(th)
	win.battery = newBatteryWork(&fakeBattery{st: st, watchOK: true}, func() {})
	g := pageContext(w, h)
	g.Metric = unit.Metric{PxPerDp: scale, PxPerSp: scale}
	paint.FillShape(g.Ops, th.UI.Active, clip.Rect{Max: image.Pt(w, h)}.Op())
	win.drawStatus(g, h-g.Dp(32), time.Now())
	if st.Present && w >= 1400 && win.statusRect(grpBattery).Empty() {
		t.Fatalf("%s %+v: battery not drawn at %d px", key, st, w)
	}
	gpu, err := headless.NewWindow(w, h)
	if err != nil {
		t.Fatal(err)
	}
	defer gpu.Release()
	if err = gpu.Frame(g.Ops); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rectangle{Max: image.Pt(w, h)})
	if err = gpu.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	return img
}

func savePNG(t *testing.T, img image.Image, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// Opt-in idle observation with the real battery source (KAZE_BATTERY_IDLE=1,
// about 4 minutes): visible, every wake asks for one frame, which ticks; then
// paused (minimized). Logs the wakes and reads; a paused reader must neither
// wake the window nor read.
func TestBatteryIdleObservation(t *testing.T) {
	if os.Getenv("KAZE_BATTERY_IDLE") == "" {
		t.Skip("set KAZE_BATTERY_IDLE=1 to watch the real battery for 4 minutes")
	}
	src := &countingSource{Source: battery.Native()}
	wakes := make(chan struct{}, 64)
	w := newBatteryWork(src, func() { wakes <- struct{}{} })
	defer w.close()
	st := w.tick(time.Now())
	t.Logf("first read: %+v, state %d (2 watching, 3 polling, 1 none)", st, w.state)
	observe := func(d time.Duration) (n int) {
		end := time.After(d)
		for {
			select {
			case <-wakes:
				n++
				w.tick(time.Now())
			case <-end:
				return n
			}
		}
	}
	reads := src.reads
	visible := observe(2 * time.Minute)
	t.Logf("visible 2 min: %d wakes, %d reads, now %+v", visible, src.reads-reads, w.status)
	w.pause(true)
	reads = src.reads
	if paused := observe(2 * time.Minute); paused != 0 || src.reads != reads {
		t.Fatalf("paused 2 min: %d wakes, %d reads", paused, src.reads-reads)
	}
	t.Log("paused 2 min: 0 wakes, 0 reads")
}

// countingSource counts reads of a real source.
type countingSource struct {
	battery.Source
	reads int
}

func (c *countingSource) Read() (battery.Status, error) { c.reads++; return c.Source.Read() }

// Opt-in development captures of the battery in the status bar:
// KAZE_BATTERY_RENDER_DIR. Per theme: the four states at 1400 px, a 700 px bar
// (battery dropped), and a 2x zoom of the right end of all four states.
func TestBatteryRenders(t *testing.T) {
	dir := os.Getenv("KAZE_BATTERY_RENDER_DIR")
	if dir == "" {
		t.Skip("set KAZE_BATTERY_RENDER_DIR for headless captures of the battery")
	}
	os.MkdirAll(dir, 0o755)
	states := []struct {
		name string
		st   battery.Status
	}{
		{"normal", battery.Status{Present: true, Percent: 82}},
		{"charging", battery.Status{Present: true, Percent: 64, Charging: true}},
		{"low", battery.Status{Present: true, Percent: 18}},
		{"verylow", battery.Status{Present: true, Percent: 7}},
	}
	for _, key := range []string{"midnight", "sumi", "washi"} {
		const zw, zh = 2800, 80
		zoom := image.NewRGBA(image.Rect(0, 0, 900, zh*len(states)))
		for i, s := range states {
			savePNG(t, batteryStrip(t, key, s.st, 1400, 120, 1), filepath.Join(dir, "battery-"+key+"-"+s.name+".png"))
			big := batteryStrip(t, key, s.st, zw, zh, 2)
			draw.Draw(zoom, image.Rect(0, i*zh, 900, (i+1)*zh), big, image.Pt(zw-900, 0), draw.Src)
		}
		savePNG(t, zoom, filepath.Join(dir, "battery-"+key+"-zoom.png"))
		savePNG(t, batteryStrip(t, key, states[0].st, 700, 120, 1), filepath.Join(dir, "battery-"+key+"-narrow.png"))
	}
}
