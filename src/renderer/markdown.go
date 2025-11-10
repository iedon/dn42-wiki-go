package renderer

import (
	"bytes"
	"fmt"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	htmlRenderer "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Heading represents a heading entry for table-of-contents rendering.
type Heading struct {
	ID    string
	Text  string
	Level int
}

// RenderResult wraps HTML markup and extracted metadata.
type RenderResult struct {
	HTML      []byte
	PlainText string
	Headings  []Heading
}

// Renderer transforms markdown sources into HTML fragments.
type Renderer struct {
	md goldmark.Markdown
}

// New constructs a renderer with GitHub-flavored markdown extensions and syntax highlighting.
func New() *Renderer {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.DefinitionList,
			extension.Footnote,
			extension.Table,
			extension.TaskList,
			extension.Typographer,
			extension.Strikethrough,
			highlighting.NewHighlighting(
				highlighting.WithFormatOptions(
					chromahtml.WithClasses(true),
					chromahtml.WithAllClasses(true),
					chromahtml.ClassPrefix("z-"),
					chromahtml.PreventSurroundingPre(true),
				),
				highlighting.WithWrapperRenderer(codeWrapper),
			),
			meta.Meta,
		),
		goldmark.WithParserOptions(
			parser.WithAttribute(),
		),
		goldmark.WithRendererOptions(
			htmlRenderer.WithUnsafe(),
		),
	)

	return &Renderer{md: md}
}

// Render converts the provided markdown into HTML and extracts metadata for navigation and search.
func (r *Renderer) Render(src []byte) (*RenderResult, error) {
	reader := text.NewReader(src)
	doc := r.md.Parser().Parse(reader)

	headings := make([]Heading, 0, 16)
	plainBuilder := &strings.Builder{}
	slugCounts := make(map[string]int)

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		switch node := n.(type) {
		case *ast.Heading:
			if entering {
				attr, _ := node.AttributeString("id")
				text := extractText(node, src)
				id := attributeToString(attr)
				if id == "" {
					base := slugify(text)
					count := slugCounts[base]
					if count > 0 {
						id = fmt.Sprintf("%s-%d", base, count)
					} else {
						id = base
					}
					slugCounts[base] = count + 1
					node.SetAttributeString("id", []byte(id))
				} else {
					slugCounts[id]++
				}
				headings = append(headings, Heading{ID: id, Text: text, Level: node.Level})
			}
		case *ast.Text:
			if entering {
				plainBuilder.Write(node.Segment.Value(src))
				plainBuilder.WriteByte(' ')
			}
		}
		return ast.WalkContinue, nil
	})

	var buf bytes.Buffer
	if err := r.md.Renderer().Render(&buf, src, doc); err != nil {
		return nil, err
	}

	return &RenderResult{HTML: buf.Bytes(), PlainText: strings.TrimSpace(plainBuilder.String()), Headings: headings}, nil
}

// MinifyHTML optimizes raw HTML markup.
// Currently a no-op, does not modify the input.
func (r *Renderer) MinifyHTML(raw []byte) ([]byte, error) {
	return raw, nil
}

func extractText(root ast.Node, source []byte) string {
	var sb strings.Builder
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if n == root {
			return ast.WalkContinue, nil
		}
		if text, ok := n.(*ast.Text); ok && entering {
			sb.Write(text.Segment.Value(source))
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(sb.String())
}

func attributeToString(value interface{}) string {
	switch v := value.(type) {
	case []byte:
		return string(v)
	case string:
		return v
	default:
		return ""
	}
}

func slugify(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return "section"
	}
	var sb strings.Builder
	lastDash := false
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_' || r == '.':
			if sb.Len() == 0 || lastDash {
				continue
			}
			sb.WriteByte('-')
			lastDash = true
		default:
			// Skip other characters
		}
	}
	slug := strings.Trim(sb.String(), "-")
	if slug == "" {
		return "section"
	}
	return slug
}

func codeWrapper(w util.BufWriter, ctx highlighting.CodeBlockContext, entering bool) {
	lang := "text"
	if raw, ok := ctx.Language(); ok && len(raw) > 0 {
		lang = string(raw)
	}
	lang = string(util.EscapeHTML([]byte(lang)))
	if entering {
		_, _ = fmt.Fprintf(w, `<pre tabindex="0" class="z-chroma z-code language-%[1]s" data-lang="%[1]s"><code class="language-%[1]s" data-lang="%[1]s">`, lang)
		return
	}
	_, _ = w.WriteString("</code></pre>\n")
}
