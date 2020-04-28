// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import "testing"

func Test_generatePass(t *testing.T) {
	t.Run("Test length", func(t *testing.T) {
		length := 16
		if got := generatePass(length); len(got) != length {
			t.Errorf("length of generatePass(%d) = %d, want %d", length, len(got), length)
		}
	})

	t.Run("Test randomness", func(t *testing.T) {
		t1 := generatePass(32)
		t2 := generatePass(32)
		if t1 == t2 {
			t.Errorf("generatePass() did not generate a unique password")
		}
	})
}
