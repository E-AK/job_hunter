package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"job-hunter-mcp/internal/domain"
	"time"
)

type Store struct{ DB *pgxpool.Pool }

func New(ctx context.Context, dsn string) (*Store, error) {
	p, e := pgxpool.New(ctx, dsn)
	if e != nil {
		return nil, e
	}
	if e = p.Ping(ctx); e != nil {
		p.Close()
		return nil, e
	}
	if _, e = p.Exec(ctx, `CREATE TABLE IF NOT EXISTS superjob_oauth_tokens (
		singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
		access_token TEXT NOT NULL,
		refresh_token TEXT NOT NULL,
		expires_at TIMESTAMPTZ NOT NULL,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); e != nil {
		p.Close()
		return nil, fmt.Errorf("ensure SuperJob OAuth token storage: %w", e)
	}
	return &Store{p}, nil
}

func (s *Store) GetSuperJobOAuthToken(ctx context.Context) (domain.SuperJobOAuthToken, bool, error) {
	var token domain.SuperJobOAuthToken
	e := s.DB.QueryRow(ctx, `SELECT access_token, refresh_token, expires_at
		FROM superjob_oauth_tokens WHERE singleton=TRUE`).Scan(&token.AccessToken, &token.RefreshToken, &token.ExpiresAt)
	if e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return domain.SuperJobOAuthToken{}, false, nil
		}
		return domain.SuperJobOAuthToken{}, false, e
	}
	return token, true, nil
}

func (s *Store) SaveSuperJobOAuthToken(ctx context.Context, token domain.SuperJobOAuthToken) error {
	_, e := s.DB.Exec(ctx, `INSERT INTO superjob_oauth_tokens(singleton, access_token, refresh_token, expires_at)
		VALUES(TRUE, $1, $2, $3)
		ON CONFLICT(singleton) DO UPDATE SET access_token=EXCLUDED.access_token,
			refresh_token=EXCLUDED.refresh_token, expires_at=EXCLUDED.expires_at, updated_at=now()`,
		token.AccessToken, token.RefreshToken, token.ExpiresAt)
	return e
}
func (s *Store) SaveVacancies(ctx context.Context, vs []domain.Vacancy) (int, error) {
	n := 0
	for _, v := range vs {
		raw, _ := json.Marshal(v.Raw)
		_, e := s.DB.Exec(ctx, `INSERT INTO vacancies(source,source_id,title,company,description,url,city,area_id,schedule,employment,salary_from,salary_to,salary_currency,published_at,remote,raw) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) ON CONFLICT(source,source_id) DO UPDATE SET title=EXCLUDED.title,company=EXCLUDED.company,description=EXCLUDED.description,url=EXCLUDED.url,city=EXCLUDED.city,schedule=EXCLUDED.schedule,employment=EXCLUDED.employment,salary_from=EXCLUDED.salary_from,salary_to=EXCLUDED.salary_to,published_at=EXCLUDED.published_at,remote=EXCLUDED.remote,raw=EXCLUDED.raw,last_seen_at=now()`, v.Source, v.SourceID, v.Title, v.Company, v.Description, v.URL, v.City, v.AreaID, v.Schedule, v.Employment, v.SalaryFrom, v.SalaryTo, v.SalaryCurrency, v.PublishedAt, v.Remote, raw)
		if e != nil {
			return n, e
		}
		n++
	}
	return n, nil
}
func (s *Store) AddResume(ctx context.Context, title, file, mime, text string, profile map[string]any) (int64, error) {
	b, e := json.Marshal(profile)
	if e != nil {
		return 0, e
	}
	var id int64
	e = s.DB.QueryRow(ctx, `INSERT INTO resumes(title,file_name,mime_type,raw_text,profile) VALUES($1,$2,$3,$4,$5) RETURNING id`, title, file, mime, text, b).Scan(&id)
	return id, e
}
func (s *Store) ListResumes(ctx context.Context) ([]map[string]any, error) {
	rows, e := s.DB.Query(ctx, `SELECT id,title,file_name,mime_type,is_active,created_at,profile FROM resumes ORDER BY created_at DESC`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var title, file, mime string
		var active bool
		var created time.Time
		var profile []byte
		if e = rows.Scan(&id, &title, &file, &mime, &active, &created, &profile); e != nil {
			return nil, e
		}
		var p any
		_ = json.Unmarshal(profile, &p)
		out = append(out, map[string]any{"id": id, "title": title, "file_name": file, "mime_type": mime, "active": active, "created_at": created, "profile": p})
	}
	return out, rows.Err()
}
func (s *Store) ListVacancies(ctx context.Context, limit int) ([]map[string]any, error) {
	rows, e := s.DB.Query(ctx, `SELECT id,source,source_id,title,company,description,url,city,schedule,employment,salary_from,salary_to,salary_currency,published_at,remote FROM vacancies ORDER BY published_at DESC NULLS LAST LIMIT $1`, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var source, sid, title, company, desc, url, city, schedule, employment, currency string
		var sf, st *int64
		var pub *time.Time
		var remote bool
		if e = rows.Scan(&id, &source, &sid, &title, &company, &desc, &url, &city, &schedule, &employment, &sf, &st, &currency, &pub, &remote); e != nil {
			return nil, e
		}
		out = append(out, map[string]any{"id": id, "source": source, "source_id": sid, "title": title, "company": company, "description": desc, "url": url, "city": city, "schedule": schedule, "employment": employment, "salary_from": sf, "salary_to": st, "currency": currency, "published_at": pub, "remote": remote})
	}
	return out, rows.Err()
}
func (s *Store) SaveApplication(ctx context.Context, vacancyID, resumeID int64, status, letter string) (int64, error) {
	var id int64
	e := s.DB.QueryRow(ctx, `INSERT INTO applications(vacancy_id,resume_id,status,cover_letter) VALUES($1,$2,$3,$4) ON CONFLICT(vacancy_id) DO UPDATE SET resume_id=EXCLUDED.resume_id,status=EXCLUDED.status,cover_letter=EXCLUDED.cover_letter,updated_at=now() RETURNING id`, vacancyID, resumeID, status, letter).Scan(&id)
	return id, e
}
func (s *Store) String() string { return fmt.Sprintf("PostgreSQL(%s)", "connected") }
