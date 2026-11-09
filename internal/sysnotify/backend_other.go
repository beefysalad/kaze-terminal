//go:build !windows && (!darwin || ios)

package sysnotify

type unavailable struct{}

func New(Callbacks) Backend                 { return unavailable{} }
func (unavailable) Post(Notification) error { return nil }
func (unavailable) Remove(string)           {}
func (unavailable) CheckPermission()        {}
func (unavailable) Retry()                  {}
func (unavailable) Status() string          { return "System notifications are unavailable on this platform" }
func (unavailable) SetWindow(uintptr)       {}
func (unavailable) Raise()                  {}
func (unavailable) Close()                  {}
