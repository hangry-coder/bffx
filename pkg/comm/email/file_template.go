package email

import (
	"bytes"
	"fmt"
	htmltpl "html/template"
	"os"
	"path/filepath"
	texttpl "text/template"
)

// RenderFileTemplates loads assets/emails/<name>.html and optional .txt from projectRoot.
func RenderFileTemplates(projectRoot, name string, data any) (subject, htmlBody, textBody string, err error) {
	base := filepath.Join(projectRoot, "assets", "emails", name)
	htmlPath := base + ".html"
	textPath := base + ".txt"

	htmlBytes, err := os.ReadFile(htmlPath)
	if err != nil {
		return "", "", "", fmt.Errorf("email template %s: %w", htmlPath, err)
	}
	ht, err := htmltpl.New(name).Parse(string(htmlBytes))
	if err != nil {
		return "", "", "", err
	}
	var htmlBuf bytes.Buffer
	if err := ht.Execute(&htmlBuf, data); err != nil {
		return "", "", "", err
	}
	htmlBody = htmlBuf.String()

	if b, err := os.ReadFile(textPath); err == nil {
		tt, err := texttpl.New(name + "_text").Parse(string(b))
		if err != nil {
			return "", "", "", err
		}
		var textBuf bytes.Buffer
		if err := tt.Execute(&textBuf, data); err != nil {
			return "", "", "", err
		}
		textBody = textBuf.String()
	}

	subject = name
	return subject, htmlBody, textBody, nil
}
