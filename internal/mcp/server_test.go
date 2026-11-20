package mcp

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"job-hunter-mcp/internal/domain"
)

type testStore struct {
	listResumesCalls int
	saveVacancies    int
	resumes          []map[string]any
}

func (s *testStore) ListResumes(context.Context) ([]map[string]any, error) {
	s.listResumesCalls++
	return s.resumes, nil
}
func (*testStore) AddResume(context.Context, string, string, string, string, map[string]any) (int64, error) {
	return 1, nil
}
func (s *testStore) SaveVacancies(_ context.Context, vacancies []domain.Vacancy) (int, error) {
	s.saveVacancies++
	return len(vacancies), nil
}
func (*testStore) ListVacancies(context.Context, int) ([]map[string]any, error) {
	return nil, nil
}
func (*testStore) SaveApplication(context.Context, int64, int64, string, string) (int64, error) {
	return 1, nil
}

type testCache struct {
	result        any
	found         bool
	getErr        error
	setErr        error
	invalidateErr error
	getCalls      int
	setCalls      int
	invalidates   int
}

func (c *testCache) Get(context.Context, string, any) (any, bool, error) {
	c.getCalls++
	return c.result, c.found, c.getErr
}
func (c *testCache) Set(context.Context, string, any, any) error {
	c.setCalls++
	return c.setErr
}
func (c *testCache) Invalidate(context.Context) error {
	c.invalidates++
	return c.invalidateErr
}

func TestCallUsesCachedReadResult(t *testing.T) {
	store := &testStore{resumes: []map[string]any{{"id": 1}}}
	cached := map[string]any{"structuredContent": []any{"cached"}}
	cache := &testCache{result: cached, found: true}
	server := &Server{Store: store, Cache: cache}

	got, err := server.call(context.Background(), map[string]any{"name": "list_resumes"})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if !reflect.DeepEqual(got, cached) || store.listResumesCalls != 0 || cache.getCalls != 1 || cache.setCalls != 0 {
		t.Fatalf("got=%#v store calls=%d cache gets=%d sets=%d", got, store.listResumesCalls, cache.getCalls, cache.setCalls)
	}
}

func TestCallCachesReadMiss(t *testing.T) {
	store := &testStore{resumes: []map[string]any{{"id": 1}}}
	cache := &testCache{}
	server := &Server{Store: store, Cache: cache}

	if _, err := server.call(context.Background(), map[string]any{"name": "list_resumes"}); err != nil {
		t.Fatalf("call: %v", err)
	}
	if store.listResumesCalls != 1 || cache.setCalls != 1 {
		t.Fatalf("store calls=%d cache sets=%d, want 1 each", store.listResumesCalls, cache.setCalls)
	}
}

func TestWriteToolsInvalidateWithoutCachingResponses(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
	}{
		{name: "save_resume", args: map[string]any{"title": "Resume", "raw_text": "text"}},
		{name: "save_application_draft", args: map[string]any{"vacancy_id": float64(1), "resume_id": float64(1), "cover_letter": "letter"}},
		{name: "search_vacancies", args: map[string]any{"query": "developer", "location": "izhevsk", "source": "unknown"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &testStore{}
			cache := &testCache{}
			server := &Server{Store: store, Cache: cache}
			if _, err := server.call(context.Background(), map[string]any{"name": tt.name, "arguments": tt.args}); err != nil {
				t.Fatalf("call: %v", err)
			}
			if cache.invalidates != 1 || cache.setCalls != 0 {
				t.Fatalf("cache invalidates=%d sets=%d, want one invalidation and no write", cache.invalidates, cache.setCalls)
			}
		})
	}
}

func TestCallReturnsCacheErrors(t *testing.T) {
	redisErr := errors.New("Redis unavailable")
	cache := &testCache{getErr: redisErr}
	store := &testStore{}
	server := &Server{Store: store, Cache: cache}
	if _, err := server.call(context.Background(), map[string]any{"name": "list_resumes"}); !errors.Is(err, redisErr) {
		t.Fatalf("read cache error = %v, want %v", err, redisErr)
	}
	if store.listResumesCalls != 0 {
		t.Fatalf("store called after cache failure: %d", store.listResumesCalls)
	}

	cache.getErr = nil
	cache.invalidateErr = redisErr
	if _, err := server.call(context.Background(), map[string]any{"name": "save_resume", "arguments": map[string]any{"title": "Resume", "raw_text": "text"}}); !errors.Is(err, redisErr) {
		t.Fatalf("invalidation error = %v, want %v", err, redisErr)
	}
}

func TestVacancyToolsExposeOnlySuperJobSource(t *testing.T) {
	wantEnums := map[string][]string{
		"search_vacancies": {"all", "superjob"},
		"get_vacancy":      {"superjob"},
	}
	seen := make(map[string]bool, len(wantEnums))
	for _, tool := range tools() {
		name, _ := tool["name"].(string)
		want, ok := wantEnums[name]
		if !ok {
			continue
		}
		seen[name] = true
		schema := tool["inputSchema"].(map[string]any)
		properties := schema["properties"].(map[string]any)
		source := properties["source"].(map[string]any)
		if got := source["enum"]; !reflect.DeepEqual(got, want) {
			t.Errorf("%s source enum = %#v, want %#v", name, got, want)
		}
	}
	for name := range wantEnums {
		if !seen[name] {
			t.Errorf("tool %q was not registered", name)
		}
	}
}
