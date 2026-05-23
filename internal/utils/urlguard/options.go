package urlguard

import "regexp"

type CheckOptions struct {
	AllowPrivate   bool
	AllowedSchemes []string
	AllowedHostsRE *regexp.Regexp
}

func defaultOptions() CheckOptions {
	return CheckOptions{
		AllowPrivate:   false,
		AllowedSchemes: []string{"http", "https"},
	}
}

func (o CheckOptions) withDefaults() CheckOptions {
	if len(o.AllowedSchemes) == 0 {
		o.AllowedSchemes = []string{"http", "https"}
	}
	return o
}
