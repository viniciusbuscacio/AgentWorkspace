package office

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func TestDocxCreateReadReplaceRoundTrip(t *testing.T) {
	data, err := NewDocx("Relatório Mensal\n\nReceita de <janeiro> & fevereiro.")
	if err != nil {
		t.Fatalf("NewDocx: %v", err)
	}
	text, err := DocxText(data)
	if err != nil {
		t.Fatalf("DocxText: %v", err)
	}
	if !strings.Contains(text, "Relatório Mensal") || !strings.Contains(text, "Receita de <janeiro> & fevereiro.") {
		t.Fatalf("text = %q", text)
	}

	replaced, count, err := ReplaceTextInDocx(data, "<janeiro> & fevereiro", "março")
	if err != nil {
		t.Fatalf("ReplaceTextInDocx: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d", count)
	}
	text, err = DocxText(replaced)
	if err != nil {
		t.Fatalf("DocxText after replace: %v", err)
	}
	if !strings.Contains(text, "Receita de março.") || strings.Contains(text, "janeiro") {
		t.Errorf("replaced text = %q", text)
	}
}

func TestReplaceTextInDocxNotFound(t *testing.T) {
	data, err := NewDocx("um texto qualquer")
	if err != nil {
		t.Fatalf("NewDocx: %v", err)
	}
	out, count, err := ReplaceTextInDocx(data, "inexistente", "x")
	if err != nil || count != 0 || out != nil {
		t.Fatalf("expected zero replacements, got out=%v count=%d err=%v", out != nil, count, err)
	}
}

func TestReplaceKeepsFormattingSiblings(t *testing.T) {
	// A hand-built document.xml with a bold run before the target run: the
	// bold run's markup must survive the replacement byte-for-byte.
	document := `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` +
		`<w:p><w:r><w:rPr><w:b/></w:rPr><w:t>Total:</w:t></w:r><w:r><w:t xml:space="preserve"> 100 reais</w:t></w:r></w:p>` +
		`</w:body></w:document>`
	replaced, count := replaceInTextNodes([]byte(document), "100 reais", "250 reais")
	if count != 1 {
		t.Fatalf("count = %d", count)
	}
	if !strings.Contains(string(replaced), "<w:rPr><w:b/></w:rPr><w:t>Total:</w:t>") {
		t.Errorf("bold run was disturbed: %s", replaced)
	}
	if !strings.Contains(string(replaced), "> 250 reais</w:t>") {
		t.Errorf("replacement missing: %s", replaced)
	}
}

func TestReplaceEscapesXML(t *testing.T) {
	data, err := NewDocx("preço atual")
	if err != nil {
		t.Fatalf("NewDocx: %v", err)
	}
	replaced, count, err := ReplaceTextInDocx(data, "preço atual", `a < b & "c"`)
	if err != nil || count != 1 {
		t.Fatalf("replace: count=%d err=%v", count, err)
	}
	text, err := DocxText(replaced)
	if err != nil {
		t.Fatalf("DocxText: %v", err)
	}
	if !strings.Contains(text, `a < b & "c"`) {
		t.Errorf("text = %q", text)
	}
}

// minimalPptx builds a two-slide deck by hand (slide XML text nodes use the
// DrawingML <a:t> element).
func minimalPptx(t *testing.T) []byte {
	t.Helper()
	slide := func(text string) string {
		return `<?xml version="1.0"?><p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" ` +
			`xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
			`<p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>` + text + `</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`
	}
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for name, content := range map[string]string{
		"[Content_Types].xml":   `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"ppt/slides/slide1.xml": slide("Título da apresentação"),
		"ppt/slides/slide2.xml": slide("Meta de vendas: 100"),
	} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return out.Bytes()
}

func TestPptxReadAndReplace(t *testing.T) {
	data := minimalPptx(t)
	slides, err := PptxText(data)
	if err != nil {
		t.Fatalf("PptxText: %v", err)
	}
	if len(slides) != 2 || !strings.Contains(slides[0], "Título") || !strings.Contains(slides[1], "Meta de vendas") {
		t.Fatalf("slides = %+v", slides)
	}

	replaced, count, err := ReplaceTextInPptx(data, "100", "250")
	if err != nil || count != 1 {
		t.Fatalf("replace: count=%d err=%v", count, err)
	}
	slides, err = PptxText(replaced)
	if err != nil {
		t.Fatalf("PptxText after replace: %v", err)
	}
	if !strings.Contains(slides[1], "Meta de vendas: 250") {
		t.Errorf("slides = %+v", slides)
	}
}

func TestXlsxCreateReadReplaceRoundTrip(t *testing.T) {
	data, err := NewXlsx([]XlsxSheet{
		{Name: "Vendas", Rows: [][]string{{"Produto", "Valor"}, {"Caneta", "2.50"}, {"Caderno", "12"}}},
		{Name: "Notas", Rows: [][]string{{"rascunho"}}},
	})
	if err != nil {
		t.Fatalf("NewXlsx: %v", err)
	}
	text, err := XlsxText(data)
	if err != nil {
		t.Fatalf("XlsxText: %v", err)
	}
	if !strings.Contains(text, "## Sheet: Vendas") || !strings.Contains(text, "Caneta\t2.5") || !strings.Contains(text, "## Sheet: Notas") {
		t.Fatalf("text = %q", text)
	}

	replaced, count, err := ReplaceTextInXlsx(data, "Caneta", "Lápis")
	if err != nil || count != 1 {
		t.Fatalf("replace: count=%d err=%v", count, err)
	}
	text, err = XlsxText(replaced)
	if err != nil {
		t.Fatalf("XlsxText after replace: %v", err)
	}
	if !strings.Contains(text, "Lápis") || strings.Contains(text, "Caneta") {
		t.Errorf("text = %q", text)
	}
}

func TestXlsxReplaceNotFound(t *testing.T) {
	data, err := NewXlsx([]XlsxSheet{{Name: "S", Rows: [][]string{{"a"}}}})
	if err != nil {
		t.Fatalf("NewXlsx: %v", err)
	}
	out, count, err := ReplaceTextInXlsx(data, "zzz", "x")
	if err != nil || count != 0 || out != nil {
		t.Fatalf("expected zero replacements, got out=%v count=%d err=%v", out != nil, count, err)
	}
}
