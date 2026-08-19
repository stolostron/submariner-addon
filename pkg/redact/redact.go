package redact

import (
	"regexp"
	"strings"
)

var fieldsToRedact = []string{
	"brokerK8sApiServer",
	"token",
	"ca.crt",
	"psk",
}

var regex *regexp.Regexp

func init() {
	var sb strings.Builder

	sb.WriteString("(\"(")
	sb.WriteString(regexp.QuoteMeta(fieldsToRedact[0]))

	for i := 1; i < len(fieldsToRedact); i++ {
		sb.WriteString("|")
		sb.WriteString(regexp.QuoteMeta(fieldsToRedact[i]))
	}

	sb.WriteString(")\"\\s*:\\s*)\"(\\\\.|[^\"\\\\])*\"")

	regex = regexp.MustCompile(sb.String())
}

func JSON(s string) string {
	return regex.ReplaceAllString(s, "$1\"##redacted##\"")
}
