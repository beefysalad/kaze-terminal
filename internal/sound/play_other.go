//go:build !windows && !darwin

package sound

func platformPlay([]byte) {}
