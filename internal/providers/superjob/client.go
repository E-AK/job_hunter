package superjob

import (
	"encoding/json"
	"fmt"
	"io"
	"job-hunter-mcp/internal/domain"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const maxLoggedErrorBodyBytes = 4096

type Client struct {
	HTTP                        *http.Client
	AppID, Token, IzhevskTownID string
}
type response struct {
	Objects []struct {
		ID         int    `json:"id"`
		Profession string `json:"profession"`
		FirmName   string `json:"firm_name"`
		Link       string `json:"link"`
		Town       struct {
			Name string `json:"title"`
		} `json:"town"`
		TypeOfWork struct {
			Title string `json:"title"`
		} `json:"type_of_work"`
		PlaceOfWork struct {
			Title string `json:"title"`
		} `json:"place_of_work"`
		PaymentFrom int64  `json:"payment_from"`
		PaymentTo   int64  `json:"payment_to"`
		Date        int64  `json:"date_published"`
		Candidat    string `json:"candidat"`
	} `json:"objects"`
}

func (c *Client) Search(p domain.SearchParams) ([]domain.Vacancy, error) {
	if c.AppID == "" {
		return nil, fmt.Errorf("SUPERJOB_APP_ID не настроен")
	}
	q := url.Values{}
	q.Set("keyword", p.Text)
	q.Set("count", strconv.Itoa(p.Limit))
	q.Set("page", strconv.Itoa(p.Page))
	if p.RemoteOnly {
		q.Set("place_of_work", "3")
	}
	if p.AreaID == "Ижевск" {
		if c.IzhevskTownID == "" {
			return nil, fmt.Errorf("для поиска SuperJob в Ижевске задайте SUPERJOB_IZHEVSK_TOWN_ID")
		}
		q.Set("town", c.IzhevskTownID)
	}
	req, e := http.NewRequest("GET", "https://api.superjob.ru/2.0/vacancies/?"+q.Encode(), nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("X-Api-App-Id", c.AppID)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	res, e := hc.Do(req)
	if e != nil {
		return nil, e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		if res.StatusCode == http.StatusForbidden {
			logForbiddenResponse("search", res.Body)
		}
		return nil, fmt.Errorf("superjob API: HTTP %d", res.StatusCode)
	}
	var d response
	if e = json.NewDecoder(res.Body).Decode(&d); e != nil {
		return nil, e
	}
	out := []domain.Vacancy{}
	for _, x := range d.Objects {
		pub := time.Unix(x.Date, 0)
		v := domain.Vacancy{Source: "superjob", SourceID: strconv.Itoa(x.ID), Title: x.Profession, Company: x.FirmName, URL: x.Link, City: x.Town.Name, Schedule: x.PlaceOfWork.Title, Employment: x.TypeOfWork.Title, SalaryCurrency: "RUR", PublishedAt: &pub, Remote: p.RemoteOnly}
		if x.PaymentFrom > 0 {
			v.SalaryFrom = &x.PaymentFrom
		}
		if x.PaymentTo > 0 {
			v.SalaryTo = &x.PaymentTo
		}
		out = append(out, v)
	}
	return out, nil
}

// Get returns the full SuperJob vacancy payload.
func (c *Client) Get(id string) (map[string]any, error) {
	if c.AppID == "" {
		return nil, fmt.Errorf("SUPERJOB_APP_ID не настроен")
	}
	req, err := http.NewRequest("GET", "https://api.superjob.ru/2.0/vacancies/"+url.PathEscape(id)+"/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-App-Id", c.AppID)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
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
		if res.StatusCode == http.StatusForbidden {
			logForbiddenResponse("get_vacancy", res.Body)
		}
		return nil, fmt.Errorf("superjob vacancy API: HTTP %d", res.StatusCode)
	}
	var out map[string]any
	if err = json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func logForbiddenResponse(operation string, body io.Reader) {
	data, err := io.ReadAll(io.LimitReader(body, maxLoggedErrorBodyBytes+1))
	if err != nil {
		log.Printf("superjob API: HTTP 403 operation=%s response body read failed: %v", operation, err)
		return
	}
	truncated := len(data) > maxLoggedErrorBodyBytes
	if truncated {
		data = data[:maxLoggedErrorBodyBytes]
	}
	log.Printf("superjob API: HTTP 403 operation=%s response_body_truncated=%t response_body=%q", operation, truncated, data)
}
