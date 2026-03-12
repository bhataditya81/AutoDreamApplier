// Package resume provides utilities for extracting text from uploaded resume files.
package resume

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

// ParseText extracts plain text from a resume file given its raw bytes and file
// extension (".pdf" or ".docx"). The returned string is suitable for keyword
// matching and LLM-based tailoring. An error is returned if the file cannot be
// parsed, but the caller should treat a non-empty error as soft (the upload can
// still succeed; raw_text will just be empty).
func ParseText(data []byte, ext string) (string, error) {
	switch strings.ToLower(ext) {
	case ".pdf":
		return parsePDF(data)
	case ".docx":
		return parseDOCX(data)
	default:
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}
}

// parsePDF extracts plain text from PDF bytes using ledongthuc/pdf.
// It iterates over every page and concatenates the plain text.
func parsePDF(data []byte) (string, error) {
	r := bytes.NewReader(data)

	reader, err := pdf.NewReader(r, int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("opening pdf reader: %w", err)
	}

	var sb strings.Builder
	numPages := reader.NumPage()

	for i := 1; i <= numPages; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			// Skip pages that fail — partial text is better than nothing.
			continue
		}

		sb.WriteString(text)

		// Ensure pages are separated even if the PDF has no trailing newline.
		if !strings.HasSuffix(text, "\n") {
			sb.WriteByte('\n')
		}
	}

	return strings.TrimSpace(sb.String()), nil
}

// parseDOCX extracts plain text from DOCX bytes.
// A DOCX file is a ZIP archive; the body text lives in "word/document.xml".
// We walk the XML token stream and collect text from <w:t> elements,
// inserting newlines at paragraph boundaries (<w:p>).
func parseDOCX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("opening docx zip: %w", err)
	}

	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("opening document.xml: %w", err)
		}
		defer rc.Close()

		return extractWordXMLText(rc)
	}

	return "", fmt.Errorf("word/document.xml not found in docx archive")
}

// extractWordXMLText parses a Word Open XML document stream and returns its
// plain text content. It handles:
//   - <w:t> — text runs (the actual character data)
//   - <w:p> — paragraph boundaries (emit a newline)
//   - <w:tab> — tab characters
//   - <w:br> — line breaks within a paragraph
func extractWordXMLText(r io.Reader) (string, error) {
	decoder := xml.NewDecoder(r)

	var sb strings.Builder
	inTextRun := false // true when we are inside a <w:t> element

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("parsing word xml: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				// <w:t> — entering a text run.
				inTextRun = true
			case "p":
				// <w:p> — new paragraph: emit a newline separator.
				sb.WriteByte('\n')
			case "tab":
				// <w:tab> — tab character.
				sb.WriteByte('\t')
			case "br":
				// <w:br> — line break within paragraph.
				sb.WriteByte('\n')
			}

		case xml.EndElement:
			if t.Name.Local == "t" {
				inTextRun = false
			}

		case xml.CharData:
			if inTextRun {
				sb.Write(t)
			}
		}
	}

	return strings.TrimSpace(sb.String()), nil
}
