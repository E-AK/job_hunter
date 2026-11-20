package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"job-hunter-mcp/internal/domain"
	"job-hunter-mcp/internal/providers/superjob"
	"net/http"
	"strings"
)

type Store interface {
	ListResumes(context.Context) ([]map[string]any, error)
	AddResume(context.Context, string, string, string, string, map[string]any) (int64, error)
	SaveVacancies(context.Context, []domain.Vacancy) (int, error)
	ListVacancies(context.Context, int) ([]map[string]any, error)
	SaveApplication(context.Context, int64, int64, string, string) (int64, error)
}

type ResultCache interface {
	Get(context.Context, string, any) (any, bool, error)
	Set(context.Context, string, any, any) error
	Invalidate(context.Context) error
}

type Server struct {
	Store                                        Store
	Cache                                        ResultCache
	SJTokenSource                                AccessTokenSource
	Token, SJAppSecret, SJToken, SJIzhevskTownID string
}

type AccessTokenSource interface {
	AccessToken(context.Context) (string, error)
}
type rpc struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params"`
}

func (s *Server) Handler() http.Handler { return http.HandlerFunc(s.handle) }
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"job-hunter-mcp"}`))
		return
	}
	if r.URL.Path != "/mcp" {
		http.NotFound(w, r)
		return
	}
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		http.Error(w, "MCP endpoint accepts POST", http.StatusMethodNotAllowed)
		return
	}
	if s.Token != "" && r.Header.Get("Authorization") != "Bearer "+s.Token {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var q rpc
	if e := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&q); e != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	if q.Method == "notifications/initialized" || strings.HasPrefix(q.Method, "notifications/") {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	var result any
	var err error
	switch q.Method {
	case "initialize":
		result = map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "job-hunter-mcp", "version": "0.1.0"}}
	case "ping":
		result = map[string]any{}
	case "tools/list":
		result = map[string]any{"tools": tools()}
	case "tools/call":
		result, err = s.call(r.Context(), q.Params)
	default:
		writeRPC(w, q.ID, nil, fmt.Errorf("method not found: %s", q.Method))
		return
	}
	writeRPC(w, q.ID, result, err)
}
func writeRPC(w http.ResponseWriter, id any, result any, err error) {
	w.Header().Set("Content-Type", "application/json")
	v := map[string]any{"jsonrpc": "2.0", "id": id}
	if err != nil {
		v["error"] = map[string]any{"code": -32000, "message": err.Error()}
	} else {
		v["result"] = result
	}
	_ = json.NewEncoder(w).Encode(v)
}
func tools() []map[string]any {
	return []map[string]any{
		tool("list_resumes", "Показать все загруженные резюме и извлечённые профили", map[string]any{"type": "object", "properties": map[string]any{}}),
		tool("save_resume", "Сохранить резюме. Передавай полный извлечённый текст и только подтверждённые факты профиля.", obj(map[string]any{"title": str(), "file_name": str(), "mime_type": str(), "raw_text": str(), "profile": map[string]any{"type": "object"}}, []string{"title", "raw_text"})),
		tool("search_vacancies", "Поиск вакансий в Ижевске (офис/гибрид/удалённо) или по России (только удалённо). Используй специализации из резюме; не добавляй навыки, которых в резюме нет.", obj(map[string]any{"query": str(), "location": map[string]any{"type": "string", "enum": []string{"izhevsk", "russia_remote"}}, "source": map[string]any{"type": "string", "enum": []string{"all", "superjob"}}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 100}}, []string{"query", "location"})),
		tool("get_vacancy", "Получить полное описание и требования конкретной вакансии", obj(map[string]any{"source": map[string]any{"type": "string", "enum": []string{"superjob"}}, "source_id": str()}, []string{"source", "source_id"})),
		tool("get_recent_vacancies", "Получить сохранённые вакансии из локальной базы", obj(map[string]any{"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 100}}, nil)),
		tool("save_application_draft", "Сохранить подготовленный отклик. Отправку работодателю этот инструмент не выполняет.", obj(map[string]any{"vacancy_id": map[string]any{"type": "integer"}, "resume_id": map[string]any{"type": "integer"}, "cover_letter": str(), "status": map[string]any{"type": "string", "enum": []string{"draft", "approved", "applied", "interview", "rejected", "offer"}}}, []string{"vacancy_id", "resume_id", "cover_letter"})),
	}
}
func tool(name, description string, schema map[string]any) map[string]any {
	return map[string]any{"name": name, "description": description, "inputSchema": schema}
}
func str() map[string]any { return map[string]any{"type": "string"} }
func obj(props map[string]any, required []string) map[string]any {
	m := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		m["required"] = required
	}
	return m
}
func arg(a map[string]any, k string) string { v, _ := a[k].(string); return v }
func num(a map[string]any, k string, def int) int {
	v, ok := a[k].(float64)
	if !ok {
		return def
	}
	return int(v)
}
func (s *Server) call(ctx context.Context, p map[string]any) (any, error) {
	name := arg(p, "name")
	a, _ := p["arguments"].(map[string]any)
	cacheable := name == "list_resumes" || name == "get_vacancy" || name == "get_recent_vacancies"
	if cacheable && s.Cache != nil {
		result, found, err := s.Cache.Get(ctx, name, a)
		if err != nil {
			return nil, fmt.Errorf("read MCP result cache: %w", err)
		}
		if found {
			return result, nil
		}
	}
	result, err := s.callTool(ctx, name, a)
	if err != nil {
		return nil, err
	}
	if cacheable && s.Cache != nil {
		if err := s.Cache.Set(ctx, name, a, result); err != nil {
			return nil, fmt.Errorf("write MCP result cache: %w", err)
		}
	}
	return result, nil
}

func (s *Server) callTool(ctx context.Context, name string, a map[string]any) (any, error) {
	switch name {
	case "list_resumes":
		v, e := s.Store.ListResumes(ctx)
		return content(v), e
	case "save_resume":
		profile, _ := a["profile"].(map[string]any)
		id, e := s.Store.AddResume(ctx, arg(a, "title"), arg(a, "file_name"), arg(a, "mime_type"), arg(a, "raw_text"), profile)
		if e == nil {
			e = s.invalidateCache(ctx)
		}
		return content(map[string]any{"resume_id": id, "message": "Резюме сохранено. Теперь можно анализировать его и подбирать вакансии."}), e
	case "search_vacancies":
		query := arg(a, "query")
		loc := arg(a, "location")
		source := arg(a, "source")
		if source == "" {
			source = "all"
		}
		limit := num(a, "limit", 30)
		if limit < 1 {
			limit = 30
		}
		if limit > 100 {
			limit = 100
		}
		area := "Ижевск"
		remote := false
		if loc == "russia_remote" {
			area = "113"
			remote = true
		}
		var providers []domain.Provider
		var errs []string
		if source == "all" || source == "superjob" {
			token, tokenErr := s.superJobAccessToken(ctx)
			if tokenErr != nil {
				errs = append(errs, tokenErr.Error())
			} else {
				providers = append(providers, &superjob.Client{
					AppSecret: s.SJAppSecret, Token: token, IzhevskTownID: s.SJIzhevskTownID,
				})
			}
		}
		var all []domain.Vacancy
		for _, pr := range providers {
			vs, e := pr.Search(domain.SearchParams{Text: query, AreaID: area, RemoteOnly: remote, Limit: limit, Page: 0})
			if e != nil {
				errs = append(errs, e.Error())
				continue
			}
			all = append(all, vs...)
		}
		saved, e := s.Store.SaveVacancies(ctx, all)
		if e != nil {
			return nil, e
		}
		if e = s.invalidateCache(ctx); e != nil {
			return nil, e
		}
		return content(map[string]any{"found": len(all), "saved_or_updated": saved, "location": loc, "remote_only": remote, "vacancies": all, "provider_warnings": errs, "note": "Поиск не гарантирует полноту выдачи. В Ижевске API-фильтры по формату работы могут отличаться; проверь формат в карточке вакансии."}), nil
	case "get_vacancy":
		source, id := arg(a, "source"), arg(a, "source_id")
		if source == "superjob" {
			token, e := s.superJobAccessToken(ctx)
			if e != nil {
				return nil, e
			}
			v, e := (&superjob.Client{
				AppSecret: s.SJAppSecret, Token: token, IzhevskTownID: s.SJIzhevskTownID,
			}).Get(id)
			return content(v), e
		}
		return nil, fmt.Errorf("unsupported vacancy source: %s", source)
	case "get_recent_vacancies":
		v, e := s.Store.ListVacancies(ctx, num(a, "limit", 30))
		return content(v), e
	case "save_application_draft":
		id, e := s.Store.SaveApplication(ctx, int64(num(a, "vacancy_id", 0)), int64(num(a, "resume_id", 0)), arg(a, "status"), arg(a, "cover_letter"))
		if e == nil {
			e = s.invalidateCache(ctx)
		}
		return content(map[string]any{"application_id": id, "saved": e == nil, "message": "Черновик сохранён; отклик не отправлялся."}), e
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *Server) superJobAccessToken(ctx context.Context) (string, error) {
	if s.SJTokenSource != nil {
		return s.SJTokenSource.AccessToken(ctx)
	}
	return s.SJToken, nil
}

func (s *Server) invalidateCache(ctx context.Context) error {
	if s.Cache == nil {
		return nil
	}
	if err := s.Cache.Invalidate(ctx); err != nil {
		return fmt.Errorf("invalidate MCP result cache: %w", err)
	}
	return nil
}

func content(v any) map[string]any {
	b, _ := json.MarshalIndent(v, "", "  ")
	return map[string]any{"content": []map[string]any{{"type": "text", "text": string(b)}}, "structuredContent": v, "isError": false}
}
