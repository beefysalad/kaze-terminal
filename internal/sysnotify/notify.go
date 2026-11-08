// Package sysnotify provides native notifications for an already running Kaze.
// New is called only by the application's explicit startup path, never tests.
package sysnotify

// Notification has an opaque per-window/per-alert ID and bounded plain text.
type Notification struct {
	ID, Title, Context, Body string
}

type Callbacks struct {
	// Activate queues selection of ID; it must not call Gio synchronously.
	Activate func(id string)
	// Changed asks for a redraw after permission or delivery status changes.
	Changed func()
	// Run schedules native work through Gio's native event loop. That loop can
	// service this work while an AppKit close callback is awaiting its event ack.
	Run func(func())
}

// Backend methods are thread-safe. Post and Remove are invoked by a serial
// worker; native callbacks may arrive on other threads. Status is UI-safe.
// Raise is called only after the controller validates an explicit user click.
type Backend interface {
	Post(Notification) error
	Remove(id string)
	CheckPermission()
	// Retry is an explicit user retry, unlike ordinary permission refresh.
	Retry()
	Status() string
	SetWindow(handle uintptr)
	Raise()
	Close()
}
