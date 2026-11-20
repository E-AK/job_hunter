package hh

import (
	"encoding/json"
	"fmt"
	"job-hunter-mcp/internal/domain"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	HTTP      *http.Client
	UserAgent string
}
type response struct {
	Items []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		URL     string `json:"alternate_url"`
		Snippet struct {
			Requirement    string `json:"requirement"`
			Responsibility string `json:"responsibility"`
		} `json:"snippet"`
		Employer struct {
			Name string `json:"name"`
		} `json:"employer"`
		Area struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"area"`
		Schedule struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"schedule"`
		Employment struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"employment"`
		Salary *struct {
			From     *int64 `json:"from"`
			To       *int64 `json:"to"`
			Currency string `json:"currency"`
		} `json:"salary"`
		PublishedAt string `json:"published_at"`
	} `json:"items"`
}

func (c *Client) Search(p domain.SearchParams) ([]domain.Vacancy, error) {
	q := url.Values{}
	q.Set("text", p.Text)
	q.Set("per_page", strconv.Itoa(p.Limit))
	q.Set("page", strconv.Itoa(p.Page))
	areaID := p.AreaID
	if areaID == "Ижевск" {
		resolved, resolveErr := c.ResolveArea("Ижевск")
		if resolveErr != nil {
			return nil, resolveErr
		}
		areaID = resolved
	}
	q.Set("area", areaID)
	if p.RemoteOnly {
		q.Set("schedule", "remote")
	}
	req, err := http.NewRequest("GET", "https://api.hh.ru/vacancies?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	res, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("hh API: HTTP %d", res.StatusCode)
	}
	var data response
	if err = json.NewDecoder(res.Body).Decode(&data); err != nil {
		return nil, err
	}
	out := make([]domain.Vacancy, 0, len(data.Items))
	for _, x := range data.Items {
		v := domain.Vacancy{Source: "hh", SourceID: x.ID, Title: x.Name, Company: x.Employer.Name, URL: x.URL, City: x.Area.Name, AreaID: x.Area.ID, Schedule: x.Schedule.Name, Employment: x.Employment.Name, SalaryCurrency: "RUR", Remote: strings.EqualFold(x.Schedule.ID, "remote"), Description: x.Snippet.Requirement + "\n" + x.Snippet.Responsibility, Raw: map[string]any{"snippet": x.Snippet}}
		if x.Salary != nil {
			v.SalaryFrom = x.Salary.From
			v.SalaryTo = x.Salary.To
			if x.Salary.Currency != "" {
				v.SalaryCurrency = x.Salary.Currency
			}
		}
		if t, e := time.Parse(time.RFC3339, x.PublishedAt); e == nil {
			v.PublishedAt = &t
		}
		out = append(out, v)
	}
	return out, nil
}

// ResolveArea loads HH area tree at runtime so city IDs are not hard-coded.
func (c *Client) ResolveArea(name string) (string, error) {
	req, err := http.NewRequest("GET", "https://api.hh.ru/areas/113", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	res, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("hh areas API: HTTP %d", res.StatusCode)
	}
	var root struct {
		ID    string            `json:"id"`
		Name  string            `json:"name"`
		Areas []json.RawMessage `json:"areas"`
	}
	if err = json.NewDecoder(res.Body).Decode(&root); err != nil {
		return "", err
	}
	var walk func(json.RawMessage) string
	walk = func(raw json.RawMessage) string {
		var node struct {
			ID    string            `json:"id"`
			Name  string            `json:"name"`
			Areas []json.RawMessage `json:"areas"`
		}
		if json.Unmarshal(raw, &node) != nil {
			return ""
		}
		if strings.EqualFold(node.Name, name) {
			return node.ID
		}
		for _, child := range node.Areas {
			if id := walk(child); id != "" {
				return id
			}
		}
		return ""
	}
	for _, child := range root.Areas {
		if id := walk(child); id != "" {
			return id, nil
		}
	}
	return "", fmt.Errorf("HH area %q not found", name)
}

// Get returns the full vacancy payload from HH.
func (c *Client) Get(id string) (map[string]any, error) {
	req, err := http.NewRequest("GET", "https://api.hh.ru/vacancies/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	res, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hh vacancy API: HTTP %d", res.StatusCode)
	}
	var out map[string]any
	if err = json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}
