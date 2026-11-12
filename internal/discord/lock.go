package discord

import "errors"

var ErrLocked = errors.New("discord: another Kaze owns the card")
