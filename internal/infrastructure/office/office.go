// Package office reads, edits and creates OOXML documents — .docx and .pptx
// via stdlib zip+XML manipulation (an OOXML file is a ZIP of XML parts), and
// .xlsx via excelize. Text edits operate on the XML text nodes (<w:t>/<a:t>)
// so the surrounding formatting is preserved untouched. Pure Go, offline, no
// external programs.
package office

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

// maxPartBytes caps a single decompressed XML part, defending against
// decompression bombs (mirrors attachmentsafe's cap strategy).
const maxPartBytes = 64 << 20

// readZipPart returns the named part's bytes from an OOXML archive.
func readZipPart(archive *zip.Reader, name string) ([]byte, error) {
	for _, file := range archive.File {
		if file.Name != name {
			continue
		}
		part, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", name, err)
		}
		defer part.Close()
		return io.ReadAll(io.LimitReader(part, maxPartBytes))
	}
	return nil, fmt.Errorf("%s is missing from the archive", name)
}

// rewriteZip rebuilds the archive with the given parts replaced, preserving
// every other entry and the original order.
func rewriteZip(data []byte, replacements map[string][]byte) ([]byte, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, file := range archive.File {
		if replaced, ok := replacements[file.Name]; ok {
			entry, err := writer.Create(file.Name)
			if err != nil {
				return nil, err
			}
			if _, err := entry.Write(replaced); err != nil {
				return nil, err
			}
			continue
		}
		part, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", file.Name, err)
		}
		entry, err := writer.Create(file.Name)
		if err == nil {
			_, err = io.Copy(entry, io.LimitReader(part, maxPartBytes))
		}
		_ = part.Close()
		if err != nil {
			return nil, fmt.Errorf("copy %s: %w", file.Name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// xmlPartText extracts the readable text of a WordprocessingML or DrawingML
// part: text nodes become text, paragraphs become lines, tabs and breaks are
// preserved.
func xmlPartText(part []byte) (text string, err error) {
	defer func() {
		if r := recover(); r != nil {
			text = ""
			err = fmt.Errorf("could not parse document XML (malformed or unsupported): %v", r)
		}
	}()
	decoder := xml.NewDecoder(bytes.NewReader(part))
	var b strings.Builder
	inText := false
	for {
		token, terr := decoder.Token()
		if terr == io.EOF {
			break
		}
		if terr != nil {
			return "", fmt.Errorf("could not parse document XML: %w", terr)
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "tab":
				b.WriteString("\t")
			case "br":
				b.WriteString("\n")
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				b.WriteString("\n")
			}
		case xml.CharData:
			if inText {
				b.Write(t)
			}
		}
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// textNodePattern matches WordprocessingML (<w:t>) and DrawingML (<a:t>) text
// nodes. Edits happen inside these nodes only, so run/paragraph formatting
// around them is never touched.
var textNodePattern = regexp.MustCompile(`(?s)(<(w|a):t(?:\s[^>]*)?>)(.*?)(</(?:w|a):t>)`)

func xmlUnescape(escaped string) string {
	var value string
	if err := xml.Unmarshal([]byte("<v>"+escaped+"</v>"), &value); err != nil {
		return escaped
	}
	return value
}

func xmlEscape(raw string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(raw))
	return b.String()
}

// replaceInTextNodes replaces oldText with newText inside every text node of
// an XML part, returning the new part and the number of replacements. A match
// must fall entirely inside one text node: OOXML splits styled text into
// separate runs, so a string spanning a formatting change cannot be replaced
// (the caller reports that limitation to the agent).
func replaceInTextNodes(part []byte, oldText, newText string) ([]byte, int) {
	count := 0
	replaced := textNodePattern.ReplaceAllFunc(part, func(node []byte) []byte {
		groups := textNodePattern.FindSubmatch(node)
		if groups == nil {
			return node
		}
		open, inner, closing := string(groups[1]), xmlUnescape(string(groups[3])), string(groups[4])
		if !strings.Contains(inner, oldText) {
			return node
		}
		count += strings.Count(inner, oldText)
		next := strings.ReplaceAll(inner, oldText, newText)
		// Leading/trailing whitespace is dropped by consumers unless the node
		// asks for space preservation.
		if strings.TrimSpace(next) != next && !strings.Contains(open, "xml:space") {
			open = strings.TrimSuffix(open, ">") + ` xml:space="preserve">`
		}
		return []byte(open + xmlEscape(next) + closing)
	})
	return replaced, count
}

// DocxText extracts the plain text of a .docx (word/document.xml).
func DocxText(data []byte) (string, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("could not open DOCX: %w", err)
	}
	part, err := readZipPart(archive, "word/document.xml")
	if err != nil {
		return "", fmt.Errorf("not a DOCX: %w", err)
	}
	return xmlPartText(part)
}

// ReplaceTextInDocx replaces oldText with newText in the document body,
// preserving formatting, and returns the rewritten file plus the number of
// replacements.
func ReplaceTextInDocx(data []byte, oldText, newText string) ([]byte, int, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, 0, fmt.Errorf("could not open DOCX: %w", err)
	}
	part, err := readZipPart(archive, "word/document.xml")
	if err != nil {
		return nil, 0, fmt.Errorf("not a DOCX: %w", err)
	}
	replaced, count := replaceInTextNodes(part, oldText, newText)
	if count == 0 {
		return nil, 0, nil
	}
	out, err := rewriteZip(data, map[string][]byte{"word/document.xml": replaced})
	if err != nil {
		return nil, 0, err
	}
	return out, count, nil
}

// NewDocx builds a minimal valid .docx whose body is one paragraph per line
// of text (blank lines become empty paragraphs).
func NewDocx(text string) ([]byte, error) {
	var body strings.Builder
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		body.WriteString("<w:p>")
		if line != "" {
			body.WriteString(`<w:r><w:t xml:space="preserve">`)
			body.WriteString(xmlEscape(line))
			body.WriteString("</w:t></w:r>")
		}
		body.WriteString("</w:p>")
	}
	document := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` +
		body.String() +
		`</w:body></w:document>`
	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
		`</Types>`
	rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
		`</Relationships>`

	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for _, part := range []struct{ name, content string }{
		{"[Content_Types].xml", contentTypes},
		{"_rels/.rels", rels},
		{"word/document.xml", document},
	} {
		entry, err := writer.Create(part.name)
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write([]byte(part.content)); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

var slidePartPattern = regexp.MustCompile(`^ppt/slides/slide(\d+)\.xml$`)

// slideParts returns the slide XML part names in slide order.
func slideParts(archive *zip.Reader) []string {
	var names []string
	for _, file := range archive.File {
		if slidePartPattern.MatchString(file.Name) {
			names = append(names, file.Name)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		return slideNumber(names[i]) < slideNumber(names[j])
	})
	return names
}

func slideNumber(name string) int {
	groups := slidePartPattern.FindStringSubmatch(name)
	number := 0
	if len(groups) == 2 {
		for _, digit := range groups[1] {
			number = number*10 + int(digit-'0')
		}
	}
	return number
}

// PptxText extracts each slide's text, in slide order.
func PptxText(data []byte) ([]string, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("could not open PPTX: %w", err)
	}
	names := slideParts(archive)
	if len(names) == 0 {
		return nil, fmt.Errorf("not a PPTX (or it has no slides)")
	}
	slides := make([]string, 0, len(names))
	for _, name := range names {
		part, err := readZipPart(archive, name)
		if err != nil {
			return nil, err
		}
		text, err := xmlPartText(part)
		if err != nil {
			return nil, err
		}
		slides = append(slides, text)
	}
	return slides, nil
}

// ReplaceTextInPptx replaces oldText with newText across all slides,
// preserving formatting, and returns the rewritten file plus the number of
// replacements.
func ReplaceTextInPptx(data []byte, oldText, newText string) ([]byte, int, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, 0, fmt.Errorf("could not open PPTX: %w", err)
	}
	names := slideParts(archive)
	if len(names) == 0 {
		return nil, 0, fmt.Errorf("not a PPTX (or it has no slides)")
	}
	replacements := map[string][]byte{}
	total := 0
	for _, name := range names {
		part, err := readZipPart(archive, name)
		if err != nil {
			return nil, 0, err
		}
		replaced, count := replaceInTextNodes(part, oldText, newText)
		if count > 0 {
			replacements[name] = replaced
			total += count
		}
	}
	if total == 0 {
		return nil, 0, nil
	}
	out, err := rewriteZip(data, replacements)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
