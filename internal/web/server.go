package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"vibeporter/internal/adapters"
	"vibeporter/internal/adapters/antigravity"
	"vibeporter/internal/adapters/claudecode"
	"vibeporter/internal/adapters/cursor"
	"vibeporter/internal/adapters/dsh"
	"vibeporter/internal/adapters/gemini"
	"vibeporter/internal/adapters/kimicode"
	"vibeporter/internal/adapters/opencode"
	"vibeporter/internal/adapters/windsurf"
	"vibeporter/internal/compact"
	"vibeporter/internal/handoff"
	"vibeporter/internal/models"
)

//go:embed static/*
var staticFS embed.FS

func staticHandler() http.Handler {
	sub, _ := fs.Sub(staticFS, "static")
	return http.FileServer(http.FS(sub))
}

var extractors = map[string]adapters.Extractor{
	"claudecode":  claudecode.NewAdapter(),
	"opencode":    opencode.NewAdapter(),
	"gemini":      gemini.NewAdapter(),
	"antigravity": antigravity.NewAdapter(),
	"ag":          antigravity.NewAdapter(),
	"kimicode":    kimicode.NewAdapter(),
	"kimi":        kimicode.NewAdapter(),
	"cursor":      cursor.NewAdapter(),
	"windsurf":    windsurf.NewAdapter(),
	"wind":        windsurf.NewAdapter(),
	"dsh":         dsh.NewAdapter(),
	"dhs":         dsh.NewAdapter(),
}

var injectors = map[string]adapters.Injector{
	"claudecode":  claudecode.NewAdapter(),
	"opencode":    opencode.NewAdapter(),
	"gemini":      gemini.NewAdapter(),
	"antigravity": antigravity.NewAdapter(),
	"ag":          antigravity.NewAdapter(),
	"kimicode":    kimicode.NewAdapter(),
	"kimi":        kimicode.NewAdapter(),
	"cursor":      cursor.NewAdapter(),
	"windsurf":    windsurf.NewAdapter(),
	"wind":        windsurf.NewAdapter(),
	"dsh":         dsh.NewAdapter(),
	"dhs":         dsh.NewAdapter(),
}

// canonicalAgents is adapters.CanonicalAgents -- see there for why this is
// one shared list rather than a copy per package. Every non-alias key of
// extractors must appear here (enforced by TestWebAgentListCoversExtractors).
func canonicalAgents() []string {
	return adapters.CanonicalAgents
}

// sameOriginOnly refuses requests driven by another site. `vibeporter serve`
// listens on loopback, but loopback is still reachable from any page the user
// has open: a cross-origin POST whose body is a "simple" content type is sent
// without a CORS preflight, so the request lands and its side effects happen
// even though the attacker cannot read the response. /api/handoff and
// /api/migrate take a caller-supplied `target` path, so without this guard a
// visited web page could write a file of its choosing anywhere the user can
// write. Requests from the bundled UI carry either no Origin or this server's
// own, and always send Content-Type: application/json.
func sameOriginOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			writeAPIError(w, http.StatusForbidden, fmt.Errorf("cross-site request refused"))
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !strings.EqualFold(u.Host, r.Host) {
				writeAPIError(w, http.StatusForbidden, fmt.Errorf("cross-origin request refused"))
				return
			}
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			// A cross-origin caller cannot set this content type without a
			// preflight, which this handler never approves.
			mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || mt != "application/json" {
				writeAPIError(w, http.StatusUnsupportedMediaType, fmt.Errorf("Content-Type: application/json required"))
				return
			}
		}
		next(w, r)
	}
}

func Serve(addr string) error {
	fmt.Printf("vibeporter web at http://%s\n", addr)
	return http.ListenAndServe(addr, newMux())
}

// newMux wires every route. All /api routes go through sameOriginOnly; keep it
// that way when adding one (TestEveryAPIRouteIsGuarded checks).
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/", staticHandler())
	mux.HandleFunc("/api/agents", sameOriginOnly(handleAgents))
	mux.HandleFunc("/api/chats", sameOriginOnly(handleChats))
	mux.HandleFunc("/api/conversation", sameOriginOnly(handleConversation))
	mux.HandleFunc("/api/search", sameOriginOnly(handleSearch))
	mux.HandleFunc("/api/diff", sameOriginOnly(handleDiff))
	mux.HandleFunc("/api/migrate", sameOriginOnly(handleMigrate))
	mux.HandleFunc("/api/handoff", sameOriginOnly(handleHandoff))
	mux.HandleFunc("/api/handoff/preview", sameOriginOnly(handleHandoffPreview))
	mux.HandleFunc("/api/stats", sameOriginOnly(handleStats))
	return mux
}

type handoffRequest struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	Compact  string `json:"compact"`
	Strategy string `json:"strategy"`
}

func handoffRequestFrom(r *http.Request) (handoffRequest, error) {
	var request handoffRequest
	if r.Method != http.MethodPost {
		return request, fmt.Errorf("POST required")
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return request, fmt.Errorf("invalid JSON: %w", err)
	}
	request.From, request.To = strings.ToLower(strings.TrimSpace(request.From)), strings.ToLower(strings.TrimSpace(request.To))
	request.Source, request.Compact = strings.TrimSpace(request.Source), strings.TrimSpace(request.Compact)
	if request.From == "" || request.To == "" || request.Source == "" || request.Compact == "" {
		return request, fmt.Errorf("from, to, source, and compact are required")
	}
	return request, nil
}

func executeHandoff(r *http.Request, dryRun bool) (handoff.Result, error) {
	request, err := handoffRequestFrom(r)
	if err != nil {
		return handoff.Result{}, err
	}
	budget, err := compact.ParseBudget(request.Compact)
	if err != nil {
		return handoff.Result{}, err
	}
	extractor, ok := extractors[request.From]
	if !ok {
		return handoff.Result{}, fmt.Errorf("unknown source agent")
	}
	injector, ok := injectors[request.To]
	if !ok {
		return handoff.Result{}, fmt.Errorf("unknown target agent")
	}
	path := request.Source
	if chats, _ := extractor.ListConversations(); len(chats) > 0 {
		for _, chat := range chats {
			if chat.ID == request.Source || chat.Path == request.Source {
				path = chat.Path
				break
			}
		}
	}
	conversation, err := extractor.Extract(path)
	if err != nil {
		return handoff.Result{}, fmt.Errorf("extracting: %w", err)
	}
	return handoff.Execute(conversation, injector, handoff.Options{SourceAgent: request.From, SourceID: conversation.ID, TargetAgent: request.To, TargetPath: request.Target, Budget: budget, Strategy: compact.Strategy(request.Strategy), DryRun: dryRun})
}

func handleHandoffPreview(w http.ResponseWriter, r *http.Request) {
	result, err := executeHandoff(r, true)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func handleHandoff(w http.ResponseWriter, r *http.Request) {
	result, err := executeHandoff(r, false)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func writeAPIError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func handleAgents(w http.ResponseWriter, r *http.Request) {
	agents := canonicalAgents()
	writeJSON(w, agents)
}

func handleChats(w http.ResponseWriter, r *http.Request) {
	agent := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("agent")))
	if agent == "" {
		// all
		var all []adapters.ChatInfo
		for _, ag := range canonicalAgents() {
			ext, ok := extractors[ag]
			if !ok {
				continue
			}
			chats, _ := ext.ListConversations()
			for i := range chats {
				chats[i].Agent = ag
			}
			all = append(all, chats...)
		}
		sort.Slice(all, func(i, j int) bool { return all[i].UpdatedAt.After(all[j].UpdatedAt) })
		if len(all) > 100 {
			all = all[:100]
		}
		writeJSON(w, all)
		return
	}
	ext, ok := extractors[agent]
	if !ok {
		http.Error(w, "unknown agent", 400)
		return
	}
	chats, err := ext.ListConversations()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, chats)
}

func handleConversation(w http.ResponseWriter, r *http.Request) {
	agent := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("agent")))
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if agent == "" || id == "" {
		http.Error(w, "agent and id required", 400)
		return
	}
	ext, ok := extractors[agent]
	if !ok {
		http.Error(w, "unknown agent", 400)
		return
	}
	// resolve id via list like resolveSource does
	chats, _ := ext.ListConversations()
	path := id
	for _, c := range chats {
		if c.ID == id || c.Path == id {
			path = c.Path
			break
		}
	}
	conv, err := ext.Extract(path)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, conv)
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	agent := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("agent")))
	if q == "" {
		http.Error(w, "q required", 400)
		return
	}
	agents := canonicalAgents()
	if agent != "" {
		agents = []string{agent}
	}
	type hit struct {
		Agent   string `json:"agent"`
		ID      string `json:"id"`
		Title   string `json:"title"`
		Project string `json:"project"`
		Path    string `json:"path"`
		Snippet string `json:"snippet"`
		Matches int    `json:"matches"`
	}
	var hits []hit
	ql := strings.ToLower(q)
	for _, ag := range agents {
		ext, ok := extractors[ag]
		if !ok {
			continue
		}
		chats, _ := ext.ListConversations()
		for _, c := range chats {
			conv, err := ext.Extract(c.Path)
			if err != nil || conv == nil {
				continue
			}
			matches, snippet := matchConv(conv, ql, q)
			if matches == 0 {
				if strings.Contains(strings.ToLower(c.Title), ql) {
					matches = 1
					snippet = c.Title
				} else {
					continue
				}
			}
			hits = append(hits, hit{Agent: ag, ID: c.ID, Title: c.Title, Project: c.Project, Path: c.Path, Snippet: snippet, Matches: matches})
			if len(hits) >= 50 {
				break
			}
		}
	}
	writeJSON(w, hits)
}

func matchConv(conv *models.Conversation, ql, qOrig string) (int, string) {
	if conv == nil {
		return 0, ""
	}
	matches := 0
	var firstSnippet string
	if strings.Contains(strings.ToLower(conv.Title), ql) {
		matches++
		firstSnippet = clip(conv.Title, qOrig, 80)
	}
	for _, m := range conv.Messages {
		if strings.Contains(strings.ToLower(m.Content), ql) {
			matches++
			if firstSnippet == "" {
				firstSnippet = clip(m.Content, qOrig, 120)
			}
		}
		for _, p := range m.EffectiveParts() {
			txt := p.Text
			if p.Kind == models.PartToolCall {
				txt = p.Name + " " + p.ArgsJSON
			}
			if strings.Contains(strings.ToLower(txt), ql) {
				matches++
				if firstSnippet == "" {
					firstSnippet = clip(txt, qOrig, 120)
				}
			}
		}
	}
	return matches, firstSnippet
}

func clip(text, query string, maxLen int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	if text == "" {
		return ""
	}
	runes := []rune(text)
	lower := []rune(strings.ToLower(text))
	ql := []rune(strings.ToLower(query))
	idx := runeIndex(lower, ql)
	if idx < 0 {
		if len(runes) > maxLen {
			return string(runes[:maxLen]) + "…"
		}
		return text
	}
	start := idx - maxLen/3
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(runes) {
		end = len(runes)
		start = end - maxLen
		if start < 0 {
			start = 0
		}
	}
	snip := strings.TrimSpace(string(runes[start:end]))
	if start > 0 {
		snip = "…" + snip
	}
	if end < len(runes) {
		snip = snip + "…"
	}
	return snip
}

func runeIndex(text, query []rune) int {
	if len(query) == 0 {
		return -1
	}
	for i := 0; i+len(query) <= len(text); i++ {
		if reflect.DeepEqual(text[i:i+len(query)], query) {
			return i
		}
	}
	return -1
}

type partMismatch struct {
	Index int          `json:"index"`
	From  *models.Part `json:"from,omitempty"`
	To    *models.Part `json:"to,omitempty"`
}

type partsDiff struct {
	Equal      bool           `json:"equal"`
	Mismatches []partMismatch `json:"mismatches,omitempty"`
}

func compareParts(from, to *models.Conversation) partsDiff {
	var fromParts, toParts []models.Part
	if from != nil {
		for _, msg := range from.Messages {
			fromParts = append(fromParts, msg.EffectiveParts()...)
		}
	}
	if to != nil {
		for _, msg := range to.Messages {
			toParts = append(toParts, msg.EffectiveParts()...)
		}
	}
	result := partsDiff{Equal: len(fromParts) == len(toParts)}
	for i := 0; i < len(fromParts) || i < len(toParts); i++ {
		if i < len(fromParts) && i < len(toParts) && reflect.DeepEqual(fromParts[i], toParts[i]) {
			continue
		}
		result.Equal = false
		mismatch := partMismatch{Index: i}
		if i < len(fromParts) {
			mismatch.From = &fromParts[i]
		}
		if i < len(toParts) {
			mismatch.To = &toParts[i]
		}
		result.Mismatches = append(result.Mismatches, mismatch)
	}
	return result
}

func handleDiff(w http.ResponseWriter, r *http.Request) {
	from := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("from")))
	to := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("to")))
	source := strings.TrimSpace(r.URL.Query().Get("source"))
	if from == "" || to == "" || source == "" {
		http.Error(w, "from, to, source required", 400)
		return
	}
	fromExt, ok := extractors[from]
	if !ok {
		http.Error(w, "unknown from", 400)
		return
	}
	toInj, ok := injectors[to]
	if !ok {
		http.Error(w, "unknown to", 400)
		return
	}
	toExt, ok := extractors[to]
	if !ok {
		http.Error(w, "unknown to extractor", 400)
		return
	}
	// resolve source like cmd does
	path := source
	if chats, _ := fromExt.ListConversations(); len(chats) > 0 {
		for _, c := range chats {
			if c.ID == source || c.Path == source {
				path = c.Path
				break
			}
		}
	}
	orig, err := fromExt.Extract(path)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// temp inject
	tmp := filepath.Join(os.TempDir(), "vibeporter-diff-"+orig.ID)
	written, err := toInj.Inject(orig, tmp)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer func() {
		_ = os.RemoveAll(tmp)
		_ = os.RemoveAll(written)
		if dir := filepath.Dir(written); dir != tmp && dir != "." && dir != "/" {
			_ = os.RemoveAll(dir)
		}
	}()
	round, err := toExt.Extract(written)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// counts
	counts := func(conv *models.Conversation) map[string]int {
		m := map[string]int{"text": 0, "thinking": 0, "tool_call": 0, "tool_result": 0, "system": 0, "messages": len(conv.Messages)}
		for _, msg := range conv.Messages {
			for _, p := range msg.EffectiveParts() {
				switch p.Kind {
				case models.PartText:
					m["text"]++
				case models.PartThinking:
					m["thinking"]++
				case models.PartToolCall:
					m["tool_call"]++
				case models.PartToolResult:
					m["tool_result"]++
				}
			}
			if msg.Role == models.RoleSystem {
				m["system"]++
			}
		}
		return m
	}
	writeJSON(w, map[string]interface{}{
		"from":      map[string]interface{}{"id": orig.ID, "title": orig.Title, "counts": counts(orig)},
		"to":        map[string]interface{}{"id": round.ID, "title": round.Title, "counts": counts(round)},
		"orig":      orig,
		"round":     round,
		"fromAgent": from,
		"toAgent":   to,
		"parts":     compareParts(orig, round),
	})
}

func handleMigrate(w http.ResponseWriter, r *http.Request) {
	from := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("from")))
	to := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("to")))
	source := strings.TrimSpace(r.URL.Query().Get("source"))
	if r.Method == "POST" {
		var body struct {
			From   string `json:"from"`
			To     string `json:"to"`
			Source string `json:"source"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.From != "" {
			from = strings.ToLower(strings.TrimSpace(body.From))
		}
		if body.To != "" {
			to = strings.ToLower(strings.TrimSpace(body.To))
		}
		if body.Source != "" {
			source = strings.TrimSpace(body.Source)
		}
	}
	if from == "" || to == "" || source == "" {
		http.Error(w, "from, to, source required", 400)
		return
	}
	fromExt, ok := extractors[from]
	if !ok {
		http.Error(w, "unknown from", 400)
		return
	}
	toInj, ok := injectors[to]
	if !ok {
		http.Error(w, "unknown to", 400)
		return
	}
	path := source
	if chats, _ := fromExt.ListConversations(); len(chats) > 0 {
		for _, c := range chats {
			if c.ID == source || c.Path == source {
				path = c.Path
				break
			}
		}
	}
	orig, err := fromExt.Extract(path)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	written, err := toInj.Inject(orig, "")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]interface{}{"from": from, "to": to, "source": source, "target": written, "title": orig.Title})
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	agents := canonicalAgents()
	var rows []map[string]interface{}
	for _, ag := range agents {
		ext, ok := extractors[ag]
		if !ok {
			continue
		}
		chats, _ := ext.ListConversations()
		messages, text, thinking, tools, chars := 0, 0, 0, 0, 0
		for _, c := range chats {
			conv, _ := ext.Extract(c.Path)
			if conv == nil {
				continue
			}
			messages += len(conv.Messages)
			for _, m := range conv.Messages {
				chars += len(m.Content)
				for _, p := range m.EffectiveParts() {
					switch p.Kind {
					case models.PartText:
						text++
					case models.PartThinking:
						thinking++
					case models.PartToolCall, models.PartToolResult:
						tools++
					}
					chars += len(p.Text) + len(p.ArgsJSON)
				}
			}
		}
		rows = append(rows, map[string]interface{}{
			"agent": ag, "chats": len(chats), "messages": messages, "text": text, "thinking": thinking, "tools": tools, "chars": chars, "tokens_est": (chars + 3) / 4,
		})
	}
	writeJSON(w, rows)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
