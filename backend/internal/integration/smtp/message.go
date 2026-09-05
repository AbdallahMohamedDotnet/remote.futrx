package smtp

import (
	"fmt"
	"mime"
	"strings"
	"time"
)

// Message is the wire representation of an outbound email: exactly the
// headers and body this package knows how to serialize. It carries its own
// From, unlike the service-layer Message, because the envelope sender is not
// implied by anything at this layer.
type Message struct {
	From    string
	To      string
	Subject string
	Body    string
}

// buildRFC5322 renders msg as a CRLF-terminated RFC 5322 message ready to
// hand to the DATA command. No Message-ID header is generated - Gmail
// assigns one on submission.
func buildRFC5322(msg Message) ([]byte, error) {
	if msg.From == "" {
		return nil, fmt.Errorf("smtp: message From is empty")
	}
	if msg.To == "" {
		return nil, fmt.Errorf("smtp: message To is empty")
	}
	for name, value := range map[string]string{"From": msg.From, "To": msg.To, "Subject": msg.Subject} {
		if strings.ContainsAny(value, "\r\n") {
			return nil, fmt.Errorf("smtp: header %s contains a line break", name)
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", msg.From)
	fmt.Fprintf(&b, "To: %s\r\n", msg.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	b.WriteString("\r\n")
	b.WriteString(msg.Body)

	return []byte(b.String()), nil
}
