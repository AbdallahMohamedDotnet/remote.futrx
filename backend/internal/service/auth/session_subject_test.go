package auth

import "testing"

func TestSignInMethodForSubject(t *testing.T) {
	if subjectLocalAdmin != "local-admin" {
		t.Fatalf("local admin subject = %q; existing sessions carry this value", subjectLocalAdmin)
	}
	cases := map[string]SignInMethod{
		"local-admin":    SignInMethodPassword,
		"google-sub-123": SignInMethodGoogle,
		"":               SignInMethodGoogle,
	}
	for subject, want := range cases {
		if got := signInMethodForSubject(subject); got != want {
			t.Errorf("signInMethodForSubject(%q) = %q, want %q", subject, got, want)
		}
	}
}
