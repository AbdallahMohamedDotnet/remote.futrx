package email

import (
	"html"
	"strings"
)

// HTMLTemplate renders a branded HTML email with the given heading and body
// text. The output uses table-based layout with all inline styles for maximum
// email-client compatibility (Gmail, Outlook, Apple Mail, Yahoo). Heading and
// body are HTML-escaped to prevent injection.
func HTMLTemplate(heading, body string) string {
	h := html.EscapeString(heading)
	b := html.EscapeString(body)

	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html>
<html lang="en" xmlns="http://www.w3.org/1999/xhtml">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta http-equiv="X-UA-Compatible" content="IE=edge">
<meta name="color-scheme" content="light dark">
<title>Remote</title>
<!--[if mso]>
<style>table,td{font-family:Arial,Helvetica,sans-serif!important}</style>
<![endif]-->
</head>
<body style="margin:0;padding:0;background-color:#0e0f12;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;-webkit-font-smoothing:antialiased;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color:#0e0f12;">
<tr><td align="center" style="padding:40px 16px;">

<!-- Card -->
<table role="presentation" width="560" cellpadding="0" cellspacing="0" border="0" style="max-width:560px;width:100%;background-color:#ffffff;border-radius:12px;overflow:hidden;">

<!-- Gradient accent bar -->
<tr><td style="height:3px;background:linear-gradient(90deg,#2f6feb,#60c8ff);font-size:0;line-height:0;">&nbsp;</td></tr>

<!-- Logo area -->
<tr><td align="center" style="padding:36px 40px 0 40px;">
<table role="presentation" cellpadding="0" cellspacing="0" border="0"><tr><td align="center">
<!-- Inline SVG logo mark: browser window with cloud -->
<img src="data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSI0OCIgaGVpZ2h0PSI0OCIgdmlld0JveD0iMCAwIDQ4IDQ4Ij48ZGVmcz48bGluZWFyR3JhZGllbnQgaWQ9ImciIHgxPSIwJSIgeTE9IjAlIiB4Mj0iMTAwJSIgeTI9IjEwMCUiPjxzdG9wIG9mZnNldD0iMCUiIHN0b3AtY29sb3I9IiMyZjZmZWIiLz48c3RvcCBvZmZzZXQ9IjEwMCUiIHN0b3AtY29sb3I9IiM2MGM4ZmYiLz48L2xpbmVhckdyYWRpZW50PjwvZGVmcz48cmVjdCB4PSI0IiB5PSI2IiB3aWR0aD0iMzYiIGhlaWdodD0iMzAiIHJ4PSI1IiBmaWxsPSJub25lIiBzdHJva2U9InVybCgjZykiIHN0cm9rZS13aWR0aD0iMy41Ii8+PGNpcmNsZSBjeD0iMTEiIGN5PSIxMi41IiByPSIxLjUiIGZpbGw9InVybCgjZykiLz48Y2lyY2xlIGN4PSIxNiIgY3k9IjEyLjUiIHI9IjEuNSIgZmlsbD0idXJsKCNnKSIvPjxjaXJjbGUgY3g9IjIxIiBjeT0iMTIuNSIgcj0iMS41IiBmaWxsPSJ1cmwoI2cpIi8+PHBhdGggZD0iTTMxIDI4YTggOCAwIDAgMC0xNiAwIiBmaWxsPSJ1cmwoI2cpIiBvcGFjaXR5PSIwLjkiLz48ZWxsaXBzZSBjeD0iMjMiIGN5PSIyNSIgcng9IjEwIiByeT0iNy41IiBmaWxsPSJ1cmwoI2cpIi8+PC9zdmc+" alt="Remote" width="48" height="48" style="display:block;border:0;width:48px;height:48px;">
</td></tr><tr><td align="center" style="padding-top:12px;">
<span style="font-size:22px;font-weight:700;color:#2f6feb;letter-spacing:-0.02em;">Remote</span>
</td></tr></table>
</td></tr>

<!-- Separator -->
<tr><td style="padding:24px 40px 0 40px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">
<tr><td style="height:1px;background-color:#e5e7eb;font-size:0;line-height:0;">&nbsp;</td></tr>
</table>
</td></tr>

<!-- Content -->
<tr><td align="center" style="padding:32px 40px 40px 40px;">
<h1 style="margin:0 0 16px 0;font-size:24px;font-weight:700;color:#1f242c;line-height:1.3;">`)
	sb.WriteString(h)
	sb.WriteString(`</h1>
<p style="margin:0;font-size:15px;line-height:1.6;color:#414852;">`)
	sb.WriteString(b)
	sb.WriteString(`</p>
</td></tr>

<!-- Footer -->
<tr><td style="background-color:#f4f5f8;border-radius:0 0 12px 12px;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0">
<tr><td align="center" style="padding:20px 40px;">
<p style="margin:0;font-size:12px;line-height:1.5;color:#8e8f99;">
Sent by Remote &middot; Powered by FutrX
</p>
</td></tr>
</table>
</td></tr>

</table>
<!-- /Card -->

</td></tr>
</table>
</body>
</html>`)

	return sb.String()
}
