package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/passwordhash/cascade-hub-bot/internal/mrboard/models"
)

var ErrAPI = errors.New("gitlab API request failed")

// maxPages bounds pagination so a misbehaving server cannot keep the poll busy forever.
const maxPages = 50

type Client struct {
	baseURL string
	token   string
	group   string
	client  *http.Client
}

func New(baseURL, token, group string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), token: token, group: group, client: &http.Client{
		Timeout: 30 * time.Second,
		// PRIVATE-TOKEN must never follow redirects to another endpoint.
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

type mergeRequest struct {
	ProjectID       int64     `json:"project_id"`
	IID             int64     `json:"iid"`
	Title           string    `json:"title"`
	Draft           bool      `json:"draft"`
	WorkInProgress  bool      `json:"work_in_progress"`
	UpdatedAt       time.Time `json:"updated_at"`
	WebURL          string    `json:"web_url"`
	Author          account   `json:"author"`
	Reviewers       []account `json:"reviewers"`
	ReferenceFields struct {
		Full string `json:"full"`
	} `json:"references"`
}

type account struct {
	Username string `json:"username"`
}

type reviewer struct {
	User  account `json:"user"`
	State string  `json:"state"`
}

func (c *Client) get(ctx context.Context, path string, query url.Values, output any) (string, error) {
	endpoint := c.baseURL + "/api/v4" + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrAPI, path)
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	response, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrAPI, path)
	}
	data, readErr := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	closeErr := response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: %s: status %d", ErrAPI, path, response.StatusCode)
	}
	if readErr != nil || closeErr != nil || json.Unmarshal(data, output) != nil {
		return "", fmt.Errorf("%w: %s: unreadable response", ErrAPI, path)
	}
	return response.Header.Get("X-Next-Page"), nil
}

// OpenMergeRequests lists the open merge requests of the group with reviewer statuses for those that have reviewers.
func (c *Client) OpenMergeRequests(ctx context.Context) ([]models.MergeRequest, error) {
	var listed []mergeRequest
	page := "1"
	for pages := 0; page != ""; pages++ {
		if pages == maxPages {
			return nil, fmt.Errorf("%w: too many merge request pages", ErrAPI)
		}
		var batch []mergeRequest
		next, err := c.get(ctx, "/groups/"+url.PathEscape(c.group)+"/merge_requests", url.Values{"state": {"opened"}, "per_page": {"100"}, "page": {page}}, &batch)
		if err != nil {
			return nil, err
		}
		listed = append(listed, batch...)
		page = next
	}
	result := make([]models.MergeRequest, 0, len(listed))
	for _, listedMR := range listed {
		mr := models.MergeRequest{
			ProjectID: listedMR.ProjectID, IID: listedMR.IID, Reference: shortReference(listedMR.ReferenceFields.Full, listedMR.IID),
			WebURL: listedMR.WebURL, Title: listedMR.Title, Author: listedMR.Author.Username, Draft: listedMR.Draft || listedMR.WorkInProgress, UpdatedAt: listedMR.UpdatedAt,
		}
		if !mr.Draft && len(listedMR.Reviewers) > 0 {
			reviewers, err := c.reviewers(ctx, mr.ProjectID, mr.IID)
			if err != nil {
				return nil, err
			}
			mr.Reviewers = reviewers
		}
		result = append(result, mr)
	}
	return result, nil
}

func (c *Client) reviewers(ctx context.Context, projectID, iid int64) ([]models.Reviewer, error) {
	var listed []reviewer
	path := "/projects/" + strconv.FormatInt(projectID, 10) + "/merge_requests/" + strconv.FormatInt(iid, 10) + "/reviewers"
	page := "1"
	for pages := 0; page != ""; pages++ {
		if pages == maxPages {
			return nil, fmt.Errorf("%w: too many reviewer pages", ErrAPI)
		}
		var batch []reviewer
		next, err := c.get(ctx, path, url.Values{"per_page": {"100"}, "page": {page}}, &batch)
		if err != nil {
			return nil, err
		}
		listed = append(listed, batch...)
		page = next
	}
	result := make([]models.Reviewer, 0, len(listed))
	for _, entry := range listed {
		state := models.ReviewerState(entry.State)
		if !state.Valid() {
			// Unknown statuses from newer GitLab versions still need someone to look at the merge request.
			state = models.StateUnreviewed
		}
		result = append(result, models.Reviewer{Username: entry.User.Username, State: state})
	}
	return result, nil
}

// shortReference turns "group/sub/project!42" into "project!42".
func shortReference(full string, iid int64) string {
	path, _, found := strings.Cut(full, "!")
	if !found || path == "" {
		return "!" + strconv.FormatInt(iid, 10)
	}
	return path[strings.LastIndex(path, "/")+1:] + "!" + strconv.FormatInt(iid, 10)
}
