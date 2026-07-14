package goxdi_test

import (
	"testing"

	"github.com/MY-RV/goxdi"
)

func TestLifetimeString(t *testing.T) {
	cases := map[goxdi.Lifetime]string{
		goxdi.Singleton: "Singleton",
		goxdi.Scoped:    "Scoped",
		goxdi.Transient: "Transient",
	}
	for l, want := range cases {
		if got := l.String(); got != want {
			t.Fatalf("%v: got %q want %q", l, got, want)
		}
	}
}
