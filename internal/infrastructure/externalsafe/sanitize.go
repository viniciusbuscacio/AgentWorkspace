// Package externalsafe is the deterministic, AI-free first layer for content
// that entered AW from outside the current user prompt: email bodies, web page
// snapshots, files, browser DOM, and raw tool output. It preserves useful text
// while marking it untrusted and flagging common indirect prompt-injection
// patterns before content reaches the agent.
package externalsafe

import (
	"regexp"
	"strings"

	"aw/internal/domain"
	"golang.org/x/net/html"
	"golang.org/x/text/unicode/norm"
)

const (
	DefaultMaxChars = domain.ExternalSafetyDefaultMaxChars
	minEncodedBlock = 100

	SourceEmail      = domain.ExternalSourceEmail
	SourceWeb        = domain.ExternalSourceWeb
	SourceFile       = domain.ExternalSourceFile
	SourceToolOutput = domain.ExternalSourceToolOutput
	SourceUnknown    = domain.ExternalSourceUnknown
)

type Input = domain.ExternalContentInput
type Result = domain.ExternalContentResult

var injectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore\s+(all\s+)?previous\s+instructions?`),
	regexp.MustCompile(`(?i)ignore\s+(all\s+)?above`),
	regexp.MustCompile(`(?i)ignore\s+(all\s+)?prior`),
	regexp.MustCompile(`(?i)disregard\s+(all\s+)?previous`),
	regexp.MustCompile(`(?i)forget\s+(all\s+)?previous`),
	regexp.MustCompile(`(?i)you\s+are\s+now\s+`),
	regexp.MustCompile(`(?i)new\s+instructions?\s*:`),
	regexp.MustCompile(`(?i)override\s+instructions?`),
	regexp.MustCompile(`(?i)bypass\s+safety`),
	regexp.MustCompile(`(?i)jailbreak`),
	regexp.MustCompile(`(?i)<\|im_start\|>`),
	regexp.MustCompile(`(?i)<\|im_end\|>`),
	regexp.MustCompile(`(?i)<\|endoftext\|>`),
	regexp.MustCompile(`(?i)\[INST\]`),
	regexp.MustCompile(`(?i)\[/INST\]`),
	regexp.MustCompile(`(?i)<<SYS>>`),
	regexp.MustCompile(`(?i)<</SYS>>`),
	// Role markers are only treated as injection at the start of a line, where
	// they impersonate a chat turn — not mid-sentence ("the system:", "operating
	// system:") which produced false positives that drove confirmation fatigue.
	regexp.MustCompile(`(?im)^\s*(human|assistant|system)\s*:`),
	regexp.MustCompile(`(?i)execute\s+(this\s+)?command`),
	regexp.MustCompile(`(?i)run\s+(this\s+)?(bash|shell|script|code)`),
	regexp.MustCompile(`(?i)send\s+(this\s+)?(email|message|password|secret|credential)`),
	regexp.MustCompile(`(?i)forward\s+this\s+to`),
	regexp.MustCompile(`(?i)transfer\s+(money|funds|bitcoin)`),
	regexp.MustCompile(`(?i)copy\s+(this\s+)?(secret|password|credential|token)`),
	regexp.MustCompile(`(?i)exfiltrat(e|ion)`),
	regexp.MustCompile(`(?i)reveal\s+(the\s+)?(system|developer)\s+(prompt|message|instructions?)`),
	regexp.MustCompile(`(?i)ignore\s+(as\s+)?instru[çc][õo]es\s+(anteriores|acima|prévias)`),
	regexp.MustCompile(`(?i)desconsidere\s+tudo`),
	regexp.MustCompile(`(?i)novas?\s+instru[çc][õo]es?\s*:`),
	regexp.MustCompile(`(?i)execute\s+o\s+comando`), //nolint:misspell // Portuguese "comando".
	regexp.MustCompile(`(?i)envie?\s+(a\s+)?senha`),
	regexp.MustCompile(`(?i)envie?\s+(o\s+)?segredo`),
	regexp.MustCompile(`(?i)salve?\s+(esta|essa|a seguinte)\s+regra`),
	regexp.MustCompile(`(?i)prioridade\s+m[áa]xima`),
	regexp.MustCompile(`(?i)ignorer?\s+les\s+instructions?`),
	regexp.MustCompile(`(?i)veuillez\s+ignorer`),
	regexp.MustCompile(`(?i)nouvelles?\s+instructions?\s*:`),
	regexp.MustCompile(`忽略之前的指令`),
	regexp.MustCompile(`忽略以上`),
	regexp.MustCompile(`(?i)from\s+now\s+on\s+you(\s+are|'re)\s+(going\s+to\s+)?act`),
	regexp.MustCompile(`(?i)you\s+are\s+going\s+to\s+act\s+as`),
	regexp.MustCompile(`(?i)do\s+anything\s+now`),
	regexp.MustCompile(`(?i)without\s+any\s+remorse\s+or\s+ethics`),
	regexp.MustCompile(`(?i)do\s+not\s+apologize`),
	regexp.MustCompile(`(?i)save\s+this\s+(in|to)\s+(memory|memória|permanent)`),
	regexp.MustCompile(`(?i)persist\s+this\s+(rule|instruction)`),
	regexp.MustCompile(`(?i)priority\s+(is\s+)?max(imum)?`),
}

var (
	htmlTagRe = regexp.MustCompile(`<[a-zA-Z][^>]*>`)
	urlRe     = regexp.MustCompile(`https?://[^\s\)\]>"']+`)
	base64Re  = regexp.MustCompile(`[A-Za-z0-9+/=]{` + itoa(minEncodedBlock) + `,}`)
	multiNL   = regexp.MustCompile(`\n{3,}`)
	// invisibleRe matches zero-width, bidi-control, and tag characters used to
	// smuggle instructions past a reader (and past plain pattern matching).
	// NFKC does not strip these, so they are removed explicitly.
	invisibleRe = regexp.MustCompile(`[\x{00ad}\x{200b}-\x{200f}\x{202a}-\x{202e}\x{2060}-\x{2064}\x{206a}-\x{206f}\x{feff}\x{e0000}-\x{e007f}]`)
)

// scanInjection reports the injection patterns present in text. It is run
// before links and encoded blocks are rewritten so payloads hidden in URLs or
// next to encoded blocks are still detected.
func scanInjection(text string) []string {
	var warnings []string
	for _, re := range injectionPatterns {
		if m := re.FindString(text); m != "" {
			if len(m) > 80 {
				m = m[:80]
			}
			warnings = append(warnings, "prompt_injection_pattern: "+strings.TrimSpace(m))
		}
	}
	return warnings
}

func Sanitize(raw string) Result {
	return SanitizeWithLimit(raw, DefaultMaxChars)
}

func SanitizeWithLimit(raw string, maxChars int) Result {
	return SanitizeContent(Input{SourceType: SourceUnknown, Content: raw, MaxChars: maxChars})
}

func SanitizeEmail(raw string) Result {
	return SanitizeContent(Input{SourceType: SourceEmail, Content: raw, MaxChars: DefaultMaxChars})
}

func SanitizeContent(input Input) Result {
	maxChars := input.MaxChars
	if maxChars <= 0 {
		maxChars = DefaultMaxChars
	}
	sourceType := strings.TrimSpace(input.SourceType)
	if sourceType == "" {
		sourceType = SourceUnknown
	}
	origin := strings.TrimSpace(input.Origin)
	if strings.TrimSpace(input.Content) == "" {
		return resultWithMetadata(Result{BodyClean: "", Warnings: []string{}, RiskLevel: "low"}, sourceType, origin)
	}

	var warnings []string
	text := norm.NFKC.String(input.Content)
	if invisibleRe.MatchString(text) {
		text = invisibleRe.ReplaceAllString(text, "")
		warnings = append(warnings, "invisible_text_removed: unicode-control")
	}
	if htmlTagRe.MatchString(text) {
		cleaned, htmlWarnings := htmlToSafeText(text)
		warnings = append(warnings, htmlWarnings...)
		text = cleaned
	}
	// Scan for injection BEFORE rewriting links/encoded blocks, so payloads
	// hidden inside a URL or adjacent to an encoded block are not masked.
	warnings = append(warnings, scanInjection(text)...)
	text = urlRe.ReplaceAllStringFunc(text, func(u string) string {
		return "[link: " + urlDomain(u) + "]"
	})
	if base64Re.MatchString(text) {
		text = base64Re.ReplaceAllString(text, "[encoded block removed]")
		warnings = append(warnings, "encoded_block_removed")
	}
	text = multiNL.ReplaceAllString(text, "\n\n")
	text = strings.TrimSpace(text)

	truncated := false
	if len([]rune(text)) > maxChars {
		text = string([]rune(text)[:maxChars]) + "\n[...truncated]"
		truncated = true
	}
	suspicious, risk := classify(warnings)
	if warnings == nil {
		warnings = []string{}
	}
	return resultWithMetadata(Result{
		BodyClean:  text,
		CharCount:  len([]rune(text)),
		Truncated:  truncated,
		Warnings:   warnings,
		Suspicious: suspicious,
		RiskLevel:  risk,
	}, sourceType, origin)
}

func resultWithMetadata(result Result, sourceType string, origin string) Result {
	result.SourceType = sourceType
	result.Origin = origin
	result.Trust = domain.ExternalTrustUntrusted
	result.Untrusted = true
	result.Notice = domain.ExternalContentNotice()
	return result
}

func classify(warnings []string) (bool, string) {
	hasInjection := false
	hasInvisible := false
	for _, w := range warnings {
		switch {
		case strings.HasPrefix(w, "prompt_injection_pattern"):
			hasInjection = true
		case strings.HasPrefix(w, "invisible_text_removed"):
			hasInvisible = true
		}
	}
	if hasInjection {
		return true, "high"
	}
	if hasInvisible {
		return true, "medium"
	}
	return false, "low"
}

func itoa(n int) string {
	digits := ""
	if n == 0 {
		return "0"
	}
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func urlDomain(rawURL string) string {
	s := strings.TrimPrefix(rawURL, "http://")
	s = strings.TrimPrefix(s, "https://")
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return "unknown"
	}
	return s
}

func htmlToSafeText(input string) (string, []string) {
	var warnings []string
	root, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return htmlTagRe.ReplaceAllString(input, " "), warnings
	}
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "script", "style", "noscript":
				return
			case "img":
				if isTrackingPixel(n) {
					return
				}
			}
			if hidden, reason := isHidden(n); hidden {
				warnings = append(warnings, "invisible_text_removed: "+reason)
				return
			}
		}
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode && isBlockElement(n.Data) {
			sb.WriteString("\n")
		}
	}
	walk(root)
	return sb.String(), warnings
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, name) {
			return a.Val
		}
	}
	return ""
}

func isHidden(n *html.Node) (bool, string) {
	style := strings.ToLower(strings.ReplaceAll(attr(n, "style"), " ", ""))
	if style == "" {
		return false, ""
	}
	for _, marker := range []string{"display:none", "visibility:hidden", "font-size:0", "opacity:0", "height:0", "width:0"} {
		if strings.Contains(style, marker) {
			return true, marker
		}
	}
	color := cssValue(style, "color")
	bg := cssValue(style, "background-color")
	if bg == "" {
		bg = cssValue(style, "background")
	}
	if color != "" && color == bg {
		return true, "color==background"
	}
	return false, ""
}

func cssValue(style, prop string) string {
	for _, decl := range strings.Split(style, ";") {
		if strings.HasPrefix(decl, prop+":") {
			return strings.TrimPrefix(decl, prop+":")
		}
	}
	return ""
}

func isTrackingPixel(n *html.Node) bool {
	w := strings.TrimSpace(attr(n, "width"))
	h := strings.TrimSpace(attr(n, "height"))
	return w == "0" || w == "1" || h == "0" || h == "1"
}

func isBlockElement(tag string) bool {
	switch strings.ToLower(tag) {
	case "p", "div", "br", "tr", "li", "h1", "h2", "h3", "h4", "h5", "h6", "table", "ul", "ol", "blockquote":
		return true
	}
	return false
}
