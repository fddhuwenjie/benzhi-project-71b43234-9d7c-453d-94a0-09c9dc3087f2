package main

import "testing"

func TestParseConfigDefaultAndExplicitAddress(t *testing.T) {
	t.Setenv("PORT", "")
	cfg, err := parseConfig(nil)
	if err != nil || cfg.address != defaultAddress {
		t.Fatalf("default config = %+v, %v", cfg, err)
	}
	cfg, err = parseConfig([]string{"-addr=127.0.0.1:19991"})
	if err != nil || cfg.address != "127.0.0.1:19991" {
		t.Fatalf("explicit config = %+v, %v", cfg, err)
	}
}

func TestParseConfigPortAndRejectWildcard(t *testing.T) {
	t.Setenv("PORT", "19123")
	cfg, err := parseConfig(nil)
	if err != nil || cfg.address != "127.0.0.1:19123" {
		t.Fatalf("PORT config = %+v, %v", cfg, err)
	}
	if _, err := parseConfig([]string{"-addr=0.0.0.0:19081"}); err == nil {
		t.Fatal("expected wildcard rejection")
	}
}
