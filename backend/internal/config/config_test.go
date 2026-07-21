package config

import "testing"

func TestCodeServerBaseURLUsesInstalledDomain(t *testing.T) {
	tests := []struct {
		base string
		want string
	}{
		{base: "https://remote.example.com", want: "https://code.remote.example.com/"},
		{base: "https://app.company.test:8443/path?ignored=yes", want: "https://code.app.company.test:8443/"},
	}
	for _, test := range tests {
		got, err := CodeServerBaseURL(test.base)
		if err != nil {
			t.Fatalf("CodeServerBaseURL(%q): %v", test.base, err)
		}
		if got != test.want {
			t.Fatalf("CodeServerBaseURL(%q) = %q, want %q", test.base, got, test.want)
		}
	}
}

func TestCodeServerBaseURLRejectsInvalidBaseURL(t *testing.T) {
	if _, err := CodeServerBaseURL("remote.example.com"); err == nil {
		t.Fatal("CodeServerBaseURL accepted a URL without scheme")
	}
}
