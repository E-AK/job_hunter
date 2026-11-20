package domain

import "time"

type Vacancy struct {
	Source         string         `json:"source"`
	SourceID       string         `json:"source_id"`
	Title          string         `json:"title"`
	Company        string         `json:"company"`
	Description    string         `json:"description"`
	URL            string         `json:"url"`
	City           string         `json:"city"`
	AreaID         string         `json:"area_id"`
	Schedule       string         `json:"schedule"`
	Employment     string         `json:"employment"`
	SalaryFrom     *int64         `json:"salary_from,omitempty"`
	SalaryTo       *int64         `json:"salary_to,omitempty"`
	SalaryCurrency string         `json:"salary_currency"`
	PublishedAt    *time.Time     `json:"published_at,omitempty"`
	Remote         bool           `json:"remote"`
	Raw            map[string]any `json:"raw,omitempty"`
}
type SearchParams struct {
	Text       string
	AreaID     string
	RemoteOnly bool
	Limit      int
	Page       int
}

type SuperJobOAuthToken struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

type Provider interface {
	Search(SearchParams) ([]Vacancy, error)
}
