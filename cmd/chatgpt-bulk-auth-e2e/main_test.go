package main

import "testing"

func TestAuthStatusLooksPresent(t *testing.T) {
	t.Parallel()

	if !authStatusLooksPresent("Stored auth: present\nAuth file: /tmp/auth.json\n") {
		t.Fatal("authStatusLooksPresent() = false, want true")
	}
	if authStatusLooksPresent("Stored auth: absent\n") {
		t.Fatal("authStatusLooksPresent() = true for absent auth, want false")
	}
}

func TestDisplayEmail(t *testing.T) {
	t.Parallel()

	if got := displayEmail(" user@example.com "); got != "user@example.com" {
		t.Fatalf("displayEmail(trimmed) = %q, want %q", got, "user@example.com")
	}
	if got := displayEmail(" \t "); got != "unknown email" {
		t.Fatalf("displayEmail(blank) = %q, want %q", got, "unknown email")
	}
}
