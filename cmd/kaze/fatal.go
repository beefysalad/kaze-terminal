package main

import "log"

// showError puts a fatal error in front of the user; tests swap it.
var showError = showErrorNative

// reportFatal logs err to stderr and shows it, since a -H=windowsgui build has no console.
func reportFatal(err error) {
	log.Printf("kaze: %v", err)
	showError("kaze", "kaze has to close:\n\n"+err.Error())
}
