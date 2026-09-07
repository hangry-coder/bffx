package comm

import (
	"github.com/hangry-coder/bffx/pkg/manifest"
	"bytes"
	"fmt"
	htmltpl "html/template"
)

type TemplateManager struct {
	templates map[string]*manifest.TemplateSpec
}

func NewTemplateManager(reg *manifest.Registry) *TemplateManager {
	tm := &TemplateManager{
		templates: make(map[string]*manifest.TemplateSpec),
	}
	for _, m := range reg.Templates {
		var spec manifest.TemplateSpec
		m.UnmarshalSpec(&spec)
		tm.templates[m.Metadata.Name] = &spec
	}
	return tm
}

func (tm *TemplateManager) Render(templateName string, channel Channel, data any) (string, string, error) {
	spec, ok := tm.templates[templateName]
	if !ok {
		return "", "", fmt.Errorf("template %s not found", templateName)
	}

	content, ok := spec.Channels[string(channel)]
	if !ok {
		// Fallback to "default" if channel specific one is missing
		content, ok = spec.Channels["default"]
		if !ok {
			return "", "", fmt.Errorf("no content for channel %s in template %s", channel, templateName)
		}
	}

	// Render Title/Subject (HTML context auto-escaping)
	titleSrc := content.Title
	if titleSrc == "" {
		titleSrc = content.Subject
	}
	titleTmpl, err := htmltpl.New("title").Parse(titleSrc)
	if err != nil {
		return "", "", err
	}

	var titleBuf bytes.Buffer
	if err := titleTmpl.Execute(&titleBuf, data); err != nil {
		return "", "", err
	}

	bodyTmpl, err := htmltpl.New("body").Parse(content.Body)
	if err != nil {
		return "", "", err
	}

	var bodyBuf bytes.Buffer
	if err := bodyTmpl.Execute(&bodyBuf, data); err != nil {
		return "", "", err
	}

	return titleBuf.String(), bodyBuf.String(), nil
}
