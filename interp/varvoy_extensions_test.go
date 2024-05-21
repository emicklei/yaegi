package interp

import (
	"testing"
)

func TestCompilePackage(t *testing.T) {
	i := New(Options{})
	i.CompilePackage(".")
}
