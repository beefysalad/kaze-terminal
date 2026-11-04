// Reads the internal battery through IOKit power sources and registers for
// the system notify alerts on battery changes. Every CF object is created
// and released here, so the Go side allocates nothing per read.

#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/ps/IOPowerSources.h>
#include <IOKit/ps/IOPSKeys.h>
#include <dispatch/dispatch.h>
#include <notify.h>

#include "iops_darwin.h"

// The percent alert is posted by powerd whenever the battery percent changes
// (its notify state holds the percent in the low byte), but the public SDK
// does not name it, so it is spelled out here. The power source alert is the
// public kIOPSNotifyPowerSource.
#define kazeNotifyPercent "com.apple.system.powersources.percent"
#include "_cgo_export.h"

static int dictInt(CFDictionaryRef d, CFStringRef key) {
	CFNumberRef n = CFDictionaryGetValue(d, key);
	int v = 0;
	if (n && CFGetTypeID(n) == CFNumberGetTypeID()) {
		CFNumberGetValue(n, kCFNumberIntType, &v);
	}
	return v;
}

static int dictBool(CFDictionaryRef d, CFStringRef key, int otherwise) {
	CFBooleanRef b = CFDictionaryGetValue(d, key);
	if (b && CFGetTypeID(b) == CFBooleanGetTypeID()) {
		return CFBooleanGetValue(b) ? 1 : 0;
	}
	return otherwise;
}

static int dictIs(CFDictionaryRef d, CFStringRef key, CFStringRef want) {
	CFStringRef s = CFDictionaryGetValue(d, key);
	return s && CFGetTypeID(s) == CFStringGetTypeID() && CFEqual(s, want);
}

kaze_battery kaze_battery_read(void) {
	kaze_battery b = {0};
	CFTypeRef info = IOPSCopyPowerSourcesInfo();
	if (!info) {
		b.err = 1;
		return b;
	}
	CFArrayRef list = IOPSCopyPowerSourcesList(info);
	if (!list) {
		CFRelease(info);
		b.err = 1;
		return b;
	}
	CFIndex n = CFArrayGetCount(list);
	for (CFIndex i = 0; i < n; i++) {
		CFDictionaryRef d = IOPSGetPowerSourceDescription(info, CFArrayGetValueAtIndex(list, i));
		if (!d || !dictIs(d, CFSTR(kIOPSTypeKey), CFSTR(kIOPSInternalBatteryType))) {
			continue;
		}
		if (!dictBool(d, CFSTR(kIOPSIsPresentKey), 1)) {
			continue;
		}
		b.found = 1;
		b.cur = dictInt(d, CFSTR(kIOPSCurrentCapacityKey));
		b.full = dictInt(d, CFSTR(kIOPSMaxCapacityKey));
		b.charging = dictBool(d, CFSTR(kIOPSIsChargingKey), 0);
		b.onAC = dictIs(d, CFSTR(kIOPSPowerSourceStateKey), CFSTR(kIOPSACPowerValue));
		break;
	}
	CFRelease(list);
	CFRelease(info);
	return b;
}

// key 0 is the percent alert, 1 the power source (AC/battery) alert; they
// match keyPercent and keySource in source_darwin.go.
kaze_battery_reg kaze_battery_register(int key) {
	kaze_battery_reg r = {0};
	const char *name = key == 0 ? kazeNotifyPercent : kIOPSNotifyPowerSource;
	dispatch_queue_t q = dispatch_get_global_queue(QOS_CLASS_UTILITY, 0);
	r.status = (int)notify_register_dispatch(name, &r.token, q, ^(int token) {
		kazeBatteryChanged();
	});
	return r;
}

int kaze_battery_cancel(int token) {
	return (int)notify_cancel(token);
}
