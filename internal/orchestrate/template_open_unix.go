//go:build unix

package orchestrate

import (
	"os"
	"syscall"
)

// A replaced special file must not block the UI before handle verification.
// Root retains confinement; O_NOFOLLOW also refuses a last-component symlink.
func openTemplateHandle(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
}
