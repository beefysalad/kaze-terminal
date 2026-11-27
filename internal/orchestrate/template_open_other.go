//go:build !unix

package orchestrate

import "os"

func openTemplateHandle(root *os.Root, name string) (*os.File, error) {
	return root.Open(name)
}
