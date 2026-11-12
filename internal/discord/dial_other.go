//go:build !darwin && !windows

package discord

import "io"

func Dial() (io.ReadWriteCloser, error) { return nil, errUnsupported }
