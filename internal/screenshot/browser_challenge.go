package screenshot

import "strings"

func detectBrowserChallenge(title, body string) bool {
	text := strings.ToLower(strings.TrimSpace(title + "\n" + body))
	for _, marker := range []string{
		"just a moment",
		"performing security verification",
		"verify you are human",
		"checking your browser",
		"security service to protect against malicious bots",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
