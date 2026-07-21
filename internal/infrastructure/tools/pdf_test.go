package tools

import "testing"

func TestExtractPDFTextRejectsMalformedPDF(t *testing.T) {
	text, err := extractPDFText([]byte("%PDF-1.7\nnot a valid pdf"))
	if err == nil {
		t.Fatalf("extractPDFText malformed PDF error = nil, text = %q", text)
	}
	if text != "" {
		t.Fatalf("extractPDFText malformed PDF text = %q, want empty", text)
	}
}

func TestAbsFloat(t *testing.T) {
	if got := absFloat(-2.5); got != 2.5 {
		t.Fatalf("absFloat(-2.5) = %v", got)
	}
	if got := absFloat(3); got != 3 {
		t.Fatalf("absFloat(3) = %v", got)
	}
}
