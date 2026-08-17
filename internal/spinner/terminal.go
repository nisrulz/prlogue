package spinner

import (
	"io"
	"os"
)

// animatable reports whether w is a terminal that can render animation.
// Animation uses only carriage returns and line-clear escapes, which every
// terminal emulator supports, so any terminal device animates and every other
// writer gets static output.
func animatable(w io.Writer) bool {
	f, ok := w.(interface{ Stat() (os.FileInfo, error) })
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
