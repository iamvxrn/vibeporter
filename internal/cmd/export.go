package cmd

import (
	"fmt"
	"html"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"vibeporter/internal/models"
)

var (
	exportFrom   string
	exportSource string
	exportFormat string
	exportOutput string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export a chat to Markdown or HTML",
	Long: `Extract a chat and render it as Markdown or HTML for sharing or docs.

Example:
  vibeporter export --from claudecode --source <id> --format markdown --output chat.md
  vibeporter export --from gemini --source ~/.gemini/tmp/.../chats/session.jsonl --format html`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		extractor, ok := extractors[exportFrom]
		if !ok {
			return fmt.Errorf("source agent %s is not supported (have: %s)", exportFrom, supportedExtractors)
		}
		resolved, err := resolveSource(extractor, exportSource)
		if err != nil {
			return fmt.Errorf("resolving source: %w", err)
		}
		conv, err := extractor.Extract(resolved)
		if err != nil {
			return fmt.Errorf("extracting: %w", err)
		}

		var out string
		switch strings.ToLower(strings.TrimSpace(exportFormat)) {
		case "md", "markdown":
			out = renderMarkdown(conv)
		case "html":
			out = renderHTML(conv)
		default:
			return fmt.Errorf("unknown format %q (expected markdown or html)", exportFormat)
		}

		if strings.TrimSpace(exportOutput) == "" || exportOutput == "-" {
			_, _ = fmt.Fprint(os.Stdout, out)
			return nil
		}
		if err := os.WriteFile(exportOutput, []byte(out), 0644); err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
		fmt.Printf("Wrote %s (%d messages)\n", exportOutput, len(conv.Messages))
		return nil
	},
}

func renderMarkdown(conv *models.Conversation) string {
	var b strings.Builder
	title := strings.TrimSpace(conv.Title)
	if title == "" {
		title = "Untitled chat"
	}
	b.WriteString("# ")
	b.WriteString(title)
	b.WriteString("\n\n")
	if conv.AgentSource != "" {
		b.WriteString(fmt.Sprintf("*Agent: %s*  \n", conv.AgentSource))
	}
	if conv.ID != "" {
		b.WriteString(fmt.Sprintf("*ID: `%s`*  \n", conv.ID))
	}
	if len(conv.Messages) > 0 {
		b.WriteString(fmt.Sprintf("*Messages: %d*  \n", len(conv.Messages)))
	}
	b.WriteString("\n---\n\n")
	for _, m := range conv.Messages {
		role := string(m.Role)
		if role == "" {
			role = "unknown"
		}
		b.WriteString("## ")
		b.WriteString(strings.Title(role))
		b.WriteString("\n\n")
		parts := m.EffectiveParts()
		if len(parts) == 0 {
			b.WriteString(strings.TrimSpace(m.Content))
			b.WriteString("\n\n")
			continue
		}
		for _, p := range parts {
			switch p.Kind {
			case models.PartText:
				b.WriteString(strings.TrimSpace(p.Text))
				b.WriteString("\n\n")
			case models.PartThinking:
				lines := strings.Split(strings.TrimSpace(p.Text), "\n")
				for _, line := range lines {
					b.WriteString("> ")
					b.WriteString(line)
					b.WriteString("\n")
				}
				b.WriteString("\n")
			case models.PartToolCall:
				name := strings.TrimSpace(p.Name)
				if name == "" {
					name = "tool"
				}
				b.WriteString(fmt.Sprintf("**Tool use: `%s`**\n\n", name))
				if strings.TrimSpace(p.ArgsJSON) != "" {
					b.WriteString("```json\n")
					b.WriteString(strings.TrimSpace(p.ArgsJSON))
					b.WriteString("\n```\n\n")
				}
			case models.PartToolResult:
				b.WriteString("**Tool result**")
				if p.IsError {
					b.WriteString(" *(error)*")
				}
				b.WriteString("\n\n")
				if strings.TrimSpace(p.Text) != "" {
					b.WriteString("```\n")
					b.WriteString(strings.TrimSpace(p.Text))
					b.WriteString("\n```\n\n")
				}
			}
		}
	}
	return b.String()
}

func renderHTML(conv *models.Conversation) string {
	var b strings.Builder
	title := html.EscapeString(strings.TrimSpace(conv.Title))
	if title == "" {
		title = "Untitled chat"
	}
	b.WriteString("<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString(fmt.Sprintf("<title>%s</title>\n", title))
	b.WriteString("<style>body{font-family:system-ui,sans-serif;max-width:800px;margin:2rem auto;padding:0 1rem} pre{background:#f6f8fa;padding:1rem;overflow:auto} blockquote{border-left:3px solid #ddd;margin:1rem 0;padding-left:1rem;color:#555} .role{font-weight:600;margin-top:2rem}</style>\n")
	b.WriteString("</head>\n<body>\n")
	b.WriteString(fmt.Sprintf("<h1>%s</h1>\n", title))
	if conv.AgentSource != "" {
		b.WriteString(fmt.Sprintf("<p><em>Agent: %s</em></p>\n", html.EscapeString(conv.AgentSource)))
	}
	if conv.ID != "" {
		b.WriteString(fmt.Sprintf("<p><em>ID: <code>%s</code></em></p>\n", html.EscapeString(conv.ID)))
	}
	b.WriteString("<hr>\n")
	for _, m := range conv.Messages {
		role := html.EscapeString(string(m.Role))
		b.WriteString(fmt.Sprintf("<div class=\"role\">%s</div>\n", role))
		parts := m.EffectiveParts()
		if len(parts) == 0 {
			b.WriteString(fmt.Sprintf("<p>%s</p>\n", html.EscapeString(strings.TrimSpace(m.Content))))
			continue
		}
		for _, p := range parts {
			switch p.Kind {
			case models.PartText:
				b.WriteString(fmt.Sprintf("<p>%s</p>\n", html.EscapeString(strings.TrimSpace(p.Text))))
			case models.PartThinking:
				b.WriteString(fmt.Sprintf("<blockquote>%s</blockquote>\n", html.EscapeString(strings.TrimSpace(p.Text))))
			case models.PartToolCall:
				name := html.EscapeString(strings.TrimSpace(p.Name))
				if name == "" {
					name = "tool"
				}
				b.WriteString(fmt.Sprintf("<p><strong>Tool use: <code>%s</code></strong></p>\n", name))
				if strings.TrimSpace(p.ArgsJSON) != "" {
					b.WriteString(fmt.Sprintf("<pre><code>%s</code></pre>\n", html.EscapeString(strings.TrimSpace(p.ArgsJSON))))
				}
			case models.PartToolResult:
				b.WriteString("<p><strong>Tool result</strong>")
				if p.IsError {
					b.WriteString(" <em>(error)</em>")
				}
				b.WriteString("</p>\n")
				if strings.TrimSpace(p.Text) != "" {
					b.WriteString(fmt.Sprintf("<pre><code>%s</code></pre>\n", html.EscapeString(strings.TrimSpace(p.Text))))
				}
			}
		}
	}
	b.WriteString("</body>\n</html>\n")
	return b.String()
}

func init() {
	exportCmd.Flags().StringVar(&exportFrom, "from", "", "Source agent")
	exportCmd.Flags().StringVar(&exportSource, "source", "", "Chat id from list, or a file path")
	exportCmd.Flags().StringVar(&exportFormat, "format", "markdown", "Output format: markdown or html")
	exportCmd.Flags().StringVar(&exportOutput, "output", "", "Output file (default stdout, use - for stdout)")
	_ = exportCmd.MarkFlagRequired("from")
	_ = exportCmd.MarkFlagRequired("source")
	rootCmd.AddCommand(exportCmd)
}
