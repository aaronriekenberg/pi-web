package templates

import (
	"path/filepath"
	"text/template"
)

const (
	templatesDirectory  = "templatefiles"
	MainTemplateFile    = "main.html"
	CommandTemplateFile = "command.html"
	ProxyTemplateFile   = "proxy.html"
	DebugTemplateFile   = "debug.html"
)

var Templates = template.Must(
	template.ParseFiles(
		filepath.Join(templatesDirectory, MainTemplateFile),
		filepath.Join(templatesDirectory, CommandTemplateFile),
		filepath.Join(templatesDirectory, ProxyTemplateFile),
		filepath.Join(templatesDirectory, DebugTemplateFile),
	))
