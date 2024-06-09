package interp

import (
	"testing"

	"github.com/traefik/yaegi/stdlib"
)

func TestCompilePackage(t *testing.T) {
	i := New(Options{})
	i.Use(stdlib.Symbols)
	_, err := i.CompilePackage("/Users/emicklei/Projects/github.com/emicklei/varvoy/todebug/hello")
	if err != nil {
		t.Fatal(err)
	}
}
