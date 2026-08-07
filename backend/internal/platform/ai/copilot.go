package ai

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gerege-systems/open-gerege-core/pkg/gemini"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxToolRounds = 4

type CopilotRequest struct {
	Prompt   string        `json:"prompt"`
	TenantID string        `json:"tenant_id"`
	Lang     string        `json:"lang,omitempty"`
	History  []ChatMessage `json:"history,omitempty"`
	Audio    *Audio        `json:"audio,omitempty"`
}
type ChatMessage struct {
	Role string `json:"role"`
	Text string `json:"text"`
}
type Audio struct {
	Mime string `json:"mime"`
	Data string `json:"data"`
}
type Step struct {
	Tool string `json:"tool"`
}
type CopilotResponse struct {
	Answer     string         `json:"answer"`
	Reply      string         `json:"reply"`
	Intent     string         `json:"intent"`
	Data       map[string]any `json:"data,omitempty"`
	Actionable []string       `json:"actionable,omitempty"`
	Steps      []Step         `json:"steps,omitempty"`
	Degraded   bool           `json:"degraded,omitempty"`
}

type generator interface {
	GenerateContent(context.Context, gemini.Request) (gemini.Response, error)
}
type CopilotService struct {
	db    *pgxpool.Pool
	chat  generator
	tts   generator
	voice string
}

func NewCopilotService(db *pgxpool.Pool) *CopilotService {
	base, key := os.Getenv("GEMINI_API_BASE"), os.Getenv("GEMINI_API_KEY")
	chatModel := os.Getenv("GEMINI_MODEL")
	ttsModel := os.Getenv("GEMINI_TTS_MODEL")
	if ttsModel == "" {
		ttsModel = "gemini-2.5-flash-preview-tts"
	}
	voice := os.Getenv("GEMINI_VOICE")
	if voice == "" {
		voice = "Kore"
	}
	return &CopilotService{db: db, chat: gemini.NewClient(base, key, chatModel), tts: gemini.NewClient(base, key, ttsModel), voice: voice}
}

func (s *CopilotService) Query(ctx context.Context, req CopilotRequest) (*CopilotResponse, error) {
	req.Prompt = strings.TrimSpace(req.Prompt)
	if req.Prompt == "" && req.Audio == nil {
		return nil, fmt.Errorf("empty prompt")
	}
	if req.Lang == "" {
		req.Lang = "mn"
	}
	if req.Audio != nil {
		if err := validateAudio(req.Audio); err != nil {
			return nil, err
		}
	}
	contents := make([]gemini.Content, 0, len(req.History)+1)
	for _, h := range req.History {
		if (h.Role == "user" || h.Role == "model") && strings.TrimSpace(h.Text) != "" {
			contents = append(contents, gemini.Content{Role: h.Role, Parts: []gemini.Part{{Text: truncate(h.Text, 4000)}}})
		}
	}
	parts := []gemini.Part{}
	if req.Prompt != "" {
		parts = append(parts, gemini.Part{Text: truncate(req.Prompt, 4000)})
	}
	if req.Audio != nil {
		parts = append(parts, gemini.Part{InlineData: &gemini.Blob{MimeType: req.Audio.Mime, Data: req.Audio.Data}})
	}
	contents = append(contents, gemini.Content{Role: "user", Parts: parts})
	system := s.systemPrompt(ctx, req.TenantID, req.Lang)
	greq := gemini.Request{SystemInstruction: &gemini.Content{Parts: []gemini.Part{{Text: system}}}, Contents: contents, Tools: []gemini.Tool{{FunctionDeclarations: toolDeclarations()}}}
	steps := []Step{}
	for round := 0; round < maxToolRounds; round++ {
		out, err := s.chat.GenerateContent(ctx, greq)
		if err != nil {
			if errors.Is(err, gemini.ErrNotConfigured) || errors.Is(err, gemini.ErrUnavailable) {
				return s.localFallback(ctx, req), nil
			}
			return nil, err
		}
		calls := out.FunctionCalls()
		if len(calls) == 0 {
			answer := out.Text()
			if answer == "" {
				return s.localFallback(ctx, req), nil
			}
			return &CopilotResponse{Answer: answer, Reply: answer, Intent: "gemini", Steps: steps}, nil
		}
		greq.Contents = append(greq.Contents, out.ModelContent())
		responses := make([]gemini.Part, 0, len(calls))
		for _, call := range calls {
			result := s.executeTool(ctx, req.TenantID, call)
			steps = append(steps, Step{Tool: call.Name})
			responses = append(responses, gemini.Part{FunctionResponse: &gemini.FunctionResponse{Name: call.Name, Response: result}})
		}
		greq.Contents = append(greq.Contents, gemini.Content{Role: "user", Parts: responses})
	}
	return s.localFallback(ctx, req), nil
}

func (s *CopilotService) systemPrompt(ctx context.Context, tenantID, lang string) string {
	scope := "You are Gerege Nexus AI Copilot. Use approved tools for live data. Never expose data from another tenant."
	instructions := "Be concise, do not invent values, and reply in language code " + lang + "."
	if s.db != nil {
		rows, err := s.db.Query(ctx, `SELECT prompt_key, content FROM ai_prompts WHERE active AND (tenant_id IS NULL OR tenant_id=$1) ORDER BY tenant_id NULLS FIRST`, tenantID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var k, v string
				if rows.Scan(&k, &v) == nil {
					if k == "scope" {
						scope = v
					}
					if k == "instructions" {
						instructions = v
					}
				}
			}
		}
	}
	return scope + "\n" + instructions
}

func toolDeclarations() []gemini.FunctionDeclaration {
	return []gemini.FunctionDeclaration{
		{Name: "erp_summary", Description: "Get current tenant product, contact, warehouse and inventory totals."},
		{Name: "search_products", Description: "Search current tenant products and stock by name or SKU.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}, "required": []string{"query"}}},
		{Name: "search_knowledge", Description: "Search approved Gerege Nexus platform knowledge.", Parameters: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}, "required": []string{"query"}}},
	}
}

func (s *CopilotService) executeTool(ctx context.Context, tenantID string, call gemini.FunctionCall) map[string]any {
	if s.db == nil {
		return map[string]any{"error": "database unavailable"}
	}
	switch call.Name {
	case "erp_summary":
		var products, contacts, warehouses int
		var stock float64
		err := s.db.QueryRow(ctx, `SELECT (SELECT count(*) FROM products WHERE tenant_id=$1 AND active), (SELECT count(*) FROM contacts WHERE tenant_id=$1 AND active), (SELECT count(*) FROM warehouses WHERE tenant_id=$1), (SELECT COALESCE(sum(quantity),0) FROM stock_levels WHERE tenant_id=$1)`, tenantID).Scan(&products, &contacts, &warehouses, &stock)
		if err != nil {
			return map[string]any{"error": "summary unavailable"}
		}
		return map[string]any{"products": products, "contacts": contacts, "warehouses": warehouses, "total_stock": stock}
	case "search_products":
		q := "%" + truncate(fmt.Sprint(call.Args["query"]), 100) + "%"
		rows, err := s.db.Query(ctx, `SELECT p.sku,p.name,p.price,COALESCE(sum(sl.quantity),0) FROM products p LEFT JOIN stock_levels sl ON sl.product_id=p.id AND sl.tenant_id=p.tenant_id WHERE p.tenant_id=$1 AND p.active AND (p.name ILIKE $2 OR p.sku ILIKE $2) GROUP BY p.id ORDER BY p.name LIMIT 10`, tenantID, q)
		if err != nil {
			return map[string]any{"error": "search unavailable"}
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var sku, name string
			var price, stock float64
			if rows.Scan(&sku, &name, &price, &stock) == nil {
				items = append(items, map[string]any{"sku": sku, "name": name, "price": price, "stock": stock})
			}
		}
		return map[string]any{"items": items}
	case "search_knowledge":
		q := "%" + truncate(fmt.Sprint(call.Args["query"]), 200) + "%"
		rows, err := s.db.Query(ctx, `SELECT title,content,source_url FROM ai_knowledge WHERE (tenant_id IS NULL OR tenant_id=$1) AND (title ILIKE $2 OR content ILIKE $2) ORDER BY tenant_id NULLS LAST,updated_at DESC LIMIT 5`, tenantID, q)
		if err != nil {
			return map[string]any{"error": "knowledge unavailable"}
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var title, content, url string
			if rows.Scan(&title, &content, &url) == nil {
				items = append(items, map[string]any{"title": title, "content": truncate(content, 1200), "source_url": url})
			}
		}
		return map[string]any{"items": items}
	default:
		return map[string]any{"error": "tool not allowed"}
	}
}

func (s *CopilotService) localFallback(ctx context.Context, req CopilotRequest) *CopilotResponse {
	intent := classifyIntent(req.Prompt)
	res := &CopilotResponse{Intent: intent, Data: map[string]any{}, Degraded: true}
	if s.db != nil && intent == "inventory_status" {
		var n float64
		_ = s.db.QueryRow(ctx, `SELECT COALESCE(SUM(quantity),0) FROM stock_levels WHERE tenant_id=$1`, req.TenantID).Scan(&n)
		res.Answer = fmt.Sprintf("Нийт агуулахын үлдэгдэл: %.2f ширхэг.", n)
		res.Data["total_stock"] = n
	} else {
		res.Answer = "AI үйлчилгээ түр боломжгүй байна. GEMINI_API_KEY тохиргоог шалгана уу."
	}
	res.Reply = res.Answer
	return res
}

func (s *CopilotService) Transcribe(ctx context.Context, audio Audio) (string, error) {
	if err := validateAudio(&audio); err != nil {
		return "", err
	}
	r, e := s.chat.GenerateContent(ctx, gemini.Request{Contents: []gemini.Content{{Role: "user", Parts: []gemini.Part{{Text: "Transcribe this audio exactly. Return only the transcript."}, {InlineData: &gemini.Blob{MimeType: audio.Mime, Data: audio.Data}}}}}})
	if e != nil {
		return "", e
	}
	return r.Text(), nil
}
func (s *CopilotService) Translate(ctx context.Context, text, target string) (string, error) {
	target = truncate(target, 12)
	r, e := s.chat.GenerateContent(ctx, gemini.Request{Contents: []gemini.Content{{Role: "user", Parts: []gemini.Part{{Text: "Translate the following text to language code " + target + ". Return only the translation:\n" + truncate(text, 8000)}}}}})
	if e != nil {
		return "", e
	}
	return r.Text(), nil
}
func (s *CopilotService) Speak(ctx context.Context, text string) (*Audio, error) {
	temp := 0.2
	r, e := s.tts.GenerateContent(ctx, gemini.Request{Contents: []gemini.Content{{Role: "user", Parts: []gemini.Part{{Text: truncate(text, 2000)}}}}, GenerationConfig: &gemini.GenerationConfig{Temperature: &temp, ResponseModalities: []string{"AUDIO"}, SpeechConfig: &gemini.SpeechConfig{VoiceConfig: &gemini.VoiceConfig{PrebuiltVoiceConfig: &gemini.PrebuiltVoiceConfig{VoiceName: s.voice}}}}})
	if e != nil {
		return nil, e
	}
	blob := r.InlineAudio()
	if blob == nil {
		return nil, errors.New("tts returned no audio")
	}
	raw, e := base64.StdEncoding.DecodeString(blob.Data)
	if e != nil {
		return nil, e
	}
	wav := gemini.PCMToWAV(raw, gemini.PCMRateFromMime(blob.MimeType))
	return &Audio{Mime: "audio/wav", Data: base64.StdEncoding.EncodeToString(wav)}, nil
}

func validateAudio(a *Audio) error {
	if a == nil {
		return errors.New("audio required")
	}
	allowed := map[string]bool{"audio/webm": true, "audio/ogg": true, "audio/wav": true, "audio/mp4": true, "audio/mpeg": true}
	mime := strings.ToLower(strings.Split(a.Mime, ";")[0])
	if !allowed[mime] {
		return errors.New("unsupported audio format")
	}
	if len(a.Data) > 950000 {
		return errors.New("audio is too large")
	}
	if _, e := base64.StdEncoding.DecodeString(a.Data); e != nil {
		return errors.New("invalid audio data")
	}
	return nil
}
func classifyIntent(p string) string {
	p = strings.ToLower(p)
	if containsAny(p, "stock", "inventory", "warehouse", "quantity", "үлдэгдэл", "агуулах") {
		return "inventory_status"
	}
	if containsAny(p, "product", "sku", "price", "catalog", "бараа") {
		return "product_count"
	}
	if containsAny(p, "contact", "customer", "vendor", "client", "харилцагч") {
		return "contacts_count"
	}
	return "general"
}
func containsAny(s string, ks ...string) bool {
	for _, k := range ks {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
