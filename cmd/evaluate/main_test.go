package main

import "testing"

func TestSubcommandFlags(t *testing.T) {
	for _, args := range [][]string{{"manifest", "-sizes", "8"}, {"regression", "-repetitions", "5"}, {"pilot", "-tier", "full"}} {
		if run(args) == nil {
			t.Fatalf("accepted unrelated flags: %v", args)
		}
	}
}
