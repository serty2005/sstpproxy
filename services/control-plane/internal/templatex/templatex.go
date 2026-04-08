package templatex

import (
	"bytes"
	"os"
	"text/template"
)

type Engine struct{}

func New() *Engine {
	return &Engine{}
}

func (e *Engine) RenderFile(path string, data any) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	tpl, err := template.New(path).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
