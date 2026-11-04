// C side of the macOS battery source (source_darwin.go, iops_darwin.c).
// Declarations only: source_darwin.go uses //export, so its preamble must not
// define anything.

#ifndef KAZE_IOPS_DARWIN_H
#define KAZE_IOPS_DARWIN_H

typedef struct {
	int err, found, charging, onAC, cur, full;
} kaze_battery;

typedef struct {
	int status, token;
} kaze_battery_reg;

kaze_battery kaze_battery_read(void);
kaze_battery_reg kaze_battery_register(int key);
int kaze_battery_cancel(int token);

#endif
