package redact

import "regexp"

var fieldsToRedact = []string{
	"brokerK8sApiServer",
	"brokerK8sApiServerToken",
	"brokerK8sCA",
	"ceIPSecPSK",
}

var regex *regexp.Regexp

func init() {
	s := "(\"(" + fieldsToRedact[0]
	for i := 1; i < len(fieldsToRedact); i++ {
		s += "|" + fieldsToRedact[i]
	}

	regex = regexp.MustCompile(s + ")\"\\s*:\\s*)\".*\"")
}

func JSON(s string) string {
	return regex.ReplaceAllString(s, "$1\"##redacted##\"")
}
