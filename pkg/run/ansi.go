package run

import "regexp"

func stripANSIEscapeCodes(s string) string {
	return ansiEscapeCodeRegexp.ReplaceAllLiteralString(s, "")
}

var ansiEscapeCodeRegexp = regexp.MustCompile(
	"[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))",
)
