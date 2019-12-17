//+build !go1.13

package dingo

import (
	"fmt"
	"strings"
)

func init() {
	fmtErrorf = func(format string, a ...interface{}) error {
		return fmt.Errorf(strings.Replace(format, "%w", "%v", 1), a...)
	}
}
