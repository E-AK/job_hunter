CREATE TABLE IF NOT EXISTS resumes (
  id BIGSERIAL PRIMARY KEY,
  title TEXT NOT NULL,
  file_name TEXT NOT NULL DEFAULT '',
  mime_type TEXT NOT NULL DEFAULT 'text/plain',
  original_path TEXT NOT NULL DEFAULT '',
  raw_text TEXT NOT NULL,
  profile JSONB NOT NULL DEFAULT '{}'::jsonb,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS resumes_active_idx ON resumes(is_active, updated_at DESC);

CREATE TABLE IF NOT EXISTS vacancies (
  id BIGSERIAL PRIMARY KEY,
  source TEXT NOT NULL,
  source_id TEXT NOT NULL,
  title TEXT NOT NULL,
  company TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  url TEXT NOT NULL,
  city TEXT NOT NULL DEFAULT '',
  area_id TEXT NOT NULL DEFAULT '',
  schedule TEXT NOT NULL DEFAULT '',
  employment TEXT NOT NULL DEFAULT '',
  salary_from BIGINT,
  salary_to BIGINT,
  salary_currency TEXT NOT NULL DEFAULT 'RUR',
  published_at TIMESTAMPTZ,
  remote BOOLEAN NOT NULL DEFAULT FALSE,
  raw JSONB NOT NULL DEFAULT '{}'::jsonb,
  first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(source, source_id)
);
CREATE INDEX IF NOT EXISTS vacancies_published_idx ON vacancies(published_at DESC);
CREATE INDEX IF NOT EXISTS vacancies_remote_idx ON vacancies(remote, published_at DESC);

CREATE TABLE IF NOT EXISTS matches (
  id BIGSERIAL PRIMARY KEY,
  vacancy_id BIGINT NOT NULL REFERENCES vacancies(id) ON DELETE CASCADE,
  resume_id BIGINT NOT NULL REFERENCES resumes(id) ON DELETE CASCADE,
  score INTEGER NOT NULL CHECK(score BETWEEN 0 AND 100),
  analysis JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(vacancy_id, resume_id)
);
CREATE TABLE IF NOT EXISTS applications (
  id BIGSERIAL PRIMARY KEY,
  vacancy_id BIGINT NOT NULL REFERENCES vacancies(id),
  resume_id BIGINT REFERENCES resumes(id),
  status TEXT NOT NULL DEFAULT 'draft',
  cover_letter TEXT NOT NULL DEFAULT '',
  notes TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(vacancy_id)
);

CREATE TABLE IF NOT EXISTS superjob_oauth_tokens (
  singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
  access_token TEXT NOT NULL,
  refresh_token TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
