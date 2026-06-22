package ping

import "testing"

func TestSelectPingTargets(t *testing.T) {
	answers := []string{"192.0.2.1", "192.0.2.2"}

	first := selectPingTargets(answers, "first")
	if len(first) != 1 || first[0] != "192.0.2.1" {
		t.Fatalf("expected only the first target, got %#v", first)
	}

	all := selectPingTargets(answers, "all")
	if len(all) != 2 {
		t.Fatalf("expected all targets, got %#v", all)
	}
}
