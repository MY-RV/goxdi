package goxdi_test

import (
	"testing"

	"github.com/MY-RV/goxdi"
)

func TestLifetimeString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		lifetime goxdi.Lifetime
		want     string
	}{
		{goxdi.Singleton, "Singleton"},
		{goxdi.Scoped, "Scoped"},
		{goxdi.Transient, "Transient"},
		{goxdi.Lifetime(99), "Unknown"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.want, func(t *testing.T) {
			t.Parallel()
			if got := tc.lifetime.String(); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
