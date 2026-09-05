package smtp

import (
	"strings"
	"testing"
)

func TestBuild(t *testing.T) {
	cases := []struct {
		name    string
		msg     Message
		wantErr bool
	}{
		{
			name: "ascii subject is not encoded",
			msg:  Message{From: "sender@example.com", To: "to@example.com", Subject: "Test email", Body: "hello"},
		},
		{
			name: "non-ascii subject is RFC 2047 encoded",
			msg:  Message{From: "sender@example.com", To: "to@example.com", Subject: "café", Body: "hello"},
		},
		{
			name:    "To with a line break is rejected",
			msg:     Message{From: "sender@example.com", To: "to@example.com\r\nBcc: evil@example.com", Subject: "x", Body: "hello"},
			wantErr: true,
		},
		{
			name:    "empty From is rejected",
			msg:     Message{From: "", To: "to@example.com", Subject: "x", Body: "hello"},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := buildRFC5322(tc.msg)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			out := string(raw)

			if !strings.Contains(out, "\r\n\r\n") {
				t.Errorf("output missing header/body CRLF separator: %q", out)
			}

			switch tc.name {
			case "ascii subject is not encoded":
				if !strings.Contains(out, "Subject: Test email\r\n") {
					t.Errorf("expected raw ascii subject, got: %q", out)
				}
			case "non-ascii subject is RFC 2047 encoded":
				if !strings.Contains(out, "=?utf-8?") {
					t.Errorf("expected an RFC 2047 encoded-word, got: %q", out)
				}
				if strings.Contains(out, "café") {
					t.Errorf("expected the raw non-ascii runes to be absent, got: %q", out)
				}
			}
		})
	}
}
