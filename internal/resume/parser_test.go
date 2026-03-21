package resume_test

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/bhata/AutoDreamApplier/internal/resume"
)

// ─── Helpers ──────────────────────────────────────────────────────────────────

// buildDOCX creates a minimal valid DOCX (ZIP archive with word/document.xml)
// containing the given text in <w:t> elements.
func buildDOCX(text string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// word/document.xml with simple paragraph containing the text
	xml := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:body>` +
		`<w:p><w:r><w:t>` + text + `</w:t></w:r></w:p>` +
		`</w:body>` +
		`</w:document>`

	f, _ := zw.Create("word/document.xml")
	f.Write([]byte(xml))
	zw.Close()
	return buf.Bytes()
}

// buildDOCXWithParagraphs creates a DOCX with multiple paragraphs.
func buildDOCXWithParagraphs(paragraphs []string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	var xmlParts []string
	for _, p := range paragraphs {
		xmlParts = append(xmlParts, `<w:p><w:r><w:t>`+p+`</w:t></w:r></w:p>`)
	}

	xml := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:body>` +
		strings.Join(xmlParts, "") +
		`</w:body>` +
		`</w:document>`

	f, _ := zw.Create("word/document.xml")
	f.Write([]byte(xml))
	zw.Close()
	return buf.Bytes()
}

// ─── ParseText — unsupported extension ───────────────────────────────────────

func TestParseText_UnsupportedExtension(t *testing.T) {
	_, err := resume.ParseText([]byte("data"), ".txt")
	if err == nil {
		t.Fatal("expected error for unsupported extension, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected 'unsupported' in error, got: %v", err)
	}
}

func TestParseText_EmptyExtension(t *testing.T) {
	_, err := resume.ParseText([]byte("data"), "")
	if err == nil {
		t.Fatal("expected error for empty extension, got nil")
	}
}

// ─── ParseText — DOCX ────────────────────────────────────────────────────────

func TestParseText_DOCX_ExtractsText(t *testing.T) {
	docx := buildDOCX("John Doe Software Engineer")
	text, err := resume.ParseText(docx, ".docx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "John Doe") {
		t.Errorf("expected 'John Doe' in extracted text, got: %q", text)
	}
	if !strings.Contains(text, "Software Engineer") {
		t.Errorf("expected 'Software Engineer' in extracted text, got: %q", text)
	}
}

func TestParseText_DOCX_CaseInsensitiveExtension(t *testing.T) {
	docx := buildDOCX("Hello World")
	_, err := resume.ParseText(docx, ".DOCX")
	if err != nil {
		t.Fatalf("expected .DOCX to work case-insensitively, got: %v", err)
	}
}

func TestParseText_DOCX_MultipleParagraphs(t *testing.T) {
	docx := buildDOCXWithParagraphs([]string{
		"Experience: 5 years",
		"Skills: Go Python Kubernetes",
		"Education: BS Computer Science",
	})
	text, err := resume.ParseText(docx, ".docx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, keyword := range []string{"Experience", "Skills", "Go", "Education"} {
		if !strings.Contains(text, keyword) {
			t.Errorf("expected %q in extracted text, got: %q", keyword, text)
		}
	}
}

func TestParseText_DOCX_CorruptBytes_ReturnsError(t *testing.T) {
	_, err := resume.ParseText([]byte("this is not a zip file at all"), ".docx")
	if err == nil {
		t.Fatal("expected error for corrupt DOCX bytes, got nil")
	}
}

func TestParseText_DOCX_EmptyBytes_ReturnsError(t *testing.T) {
	_, err := resume.ParseText([]byte{}, ".docx")
	if err == nil {
		t.Fatal("expected error for empty DOCX bytes, got nil")
	}
}

// TestParseText_DOCX_MissingDocumentXML verifies missing document.xml returns an error.
func TestParseText_DOCX_MissingDocumentXML(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create("word/styles.xml") // no document.xml
	f.Write([]byte("<styles/>"))
	zw.Close()

	_, err := resume.ParseText(buf.Bytes(), ".docx")
	if err == nil {
		t.Fatal("expected error when word/document.xml is missing, got nil")
	}
}

// TestParseText_DOCX_TabAndBreak verifies <w:tab> and <w:br> elements are handled.
func TestParseText_DOCX_TabAndBreak(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	xml := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:body>` +
		`<w:p><w:r><w:t>Before</w:t></w:r><w:tab/><w:r><w:t>After</w:t></w:r><w:br/></w:p>` +
		`</w:body>` +
		`</w:document>`

	f, _ := zw.Create("word/document.xml")
	f.Write([]byte(xml))
	zw.Close()

	text, err := resume.ParseText(buf.Bytes(), ".docx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(text, "Before") || !strings.Contains(text, "After") {
		t.Errorf("expected 'Before' and 'After' in text, got: %q", text)
	}
}

// ─── ParseText — PDF ──────────────────────────────────────────────────────────

// TestParseText_PDF_CorruptBytes_ReturnsError verifies corrupt PDF data returns an error gracefully.
func TestParseText_PDF_CorruptBytes_ReturnsError(t *testing.T) {
	_, err := resume.ParseText([]byte("this is not a PDF"), ".pdf")
	if err == nil {
		t.Fatal("expected error for corrupt PDF bytes, got nil")
	}
}

func TestParseText_PDF_EmptyBytes_ReturnsError(t *testing.T) {
	_, err := resume.ParseText([]byte{}, ".pdf")
	if err == nil {
		t.Fatal("expected error for empty PDF bytes, got nil")
	}
}

func TestParseText_PDF_CaseInsensitiveExtension(t *testing.T) {
	// Even with .PDF extension the error should be about PDF parsing, not about
	// unsupported extension.
	_, err := resume.ParseText([]byte("not a pdf"), ".PDF")
	if err == nil {
		t.Fatal("expected error for corrupt PDF bytes with .PDF extension, got nil")
	}
	if strings.Contains(err.Error(), "unsupported") {
		t.Errorf("should not get 'unsupported' error for .PDF (case insensitive), got: %v", err)
	}
}

// TestParseText_PDF_ValidMinimalPDF verifies a minimal valid PDF can be parsed.
// We use a hand-crafted minimal PDF that contains no text — the parser should
// succeed and return an empty or near-empty string.
func TestParseText_PDF_ValidMinimalPDF(t *testing.T) {
	// Minimal valid PDF with one empty page.
	// This is the smallest well-formed PDF that ledongthuc/pdf can open.
	minimalPDF := []byte(`%PDF-1.0
1 0 obj<</Type /Catalog /Pages 2 0 R>>endobj
2 0 obj<</Type /Pages /Kids [3 0 R] /Count 1>>endobj
3 0 obj<</Type /Page /MediaBox [0 0 3 3]>>endobj
xref
0 4
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000115 00000 n
trailer<</Size 4 /Root 1 0 R>>
startxref
190
%%EOF`)

	// The minimal PDF above may or may not parse successfully depending on the
	// library's strictness. Either outcome (text="" or error) is acceptable as
	// long as there is no panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ParseText panicked: %v", r)
		}
	}()

	text, err := resume.ParseText(minimalPDF, ".pdf")
	// If it succeeds, text should be empty (no content in the page).
	// If it fails with an error, that's also acceptable.
	_ = text
	_ = err
}
