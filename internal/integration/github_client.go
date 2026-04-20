package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// CommitFetcher retrieves commit messages from a VCS repository.
type CommitFetcher interface {
	ListCommitMessages(ownerRepo, author string, since, until time.Time) ([]CommitInfo, error)
}

// CommitFetcherFactory creates a CommitFetcher authenticated with the given token.
type CommitFetcherFactory interface {
	New(token string) CommitFetcher
}

// GitHubClientFactory implements CommitFetcherFactory.
type GitHubClientFactory struct{}

func (GitHubClientFactory) New(token string) CommitFetcher {
	return NewGitHubClient(token)
}

// GitHubClient provides simple helpers to call GitHub APIs with a user token.
type GitHubClient struct {
	client  *http.Client
	baseURL string
	token   string
}

// NewGitHubClient creates a new GitHubClient using the provided token.
func NewGitHubClient(token string) *GitHubClient {
	return &GitHubClient{
		client:  &http.Client{Timeout: 20 * time.Second},
		baseURL: "https://api.github.com",
		token:   token,
	}
}

type ghRepo struct {
	FullName string `json:"full_name"`
}

// ListRepositories returns the authenticated user's repositories as owner/name.
func (g *GitHubClient) ListRepositories() ([]string, error) {
	var out []string
	perPage := 100
	for page := 1; ; page++ {
		u := fmt.Sprintf("%s/user/repos?per_page=%d&page=%d", g.baseURL, perPage, page)
		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		if g.token != "" {
			req.Header.Set("Authorization", "token "+g.token)
		}
		req.Header.Set("User-Agent", "bragdoc-app")

		resp, err := g.client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("github list repos status=%d: %s", resp.StatusCode, string(b))
		}

		var repos []ghRepo
		if err := json.NewDecoder(resp.Body).Decode(&repos); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()
		if len(repos) == 0 {
			break
		}
		for _, r := range repos {
			out = append(out, r.FullName)
		}
		if len(repos) < perPage {
			break
		}
	}
	return out, nil
}

// CountCommits estimates the number of commits by using pagination info.
// ownerRepo must be "owner/repo". Author is the GitHub login (author filter).
func (g *GitHubClient) CountCommits(ownerRepo, author string, since, until time.Time) (int, error) {
	// If no author filter is provided, use the fast per_page=1 trick.
	if author == "" {
		u := fmt.Sprintf("%s/repos/%s/commits?per_page=1", g.baseURL, ownerRepo)
		params := url.Values{}
		if !since.IsZero() {
			params.Set("since", since.UTC().Format(time.RFC3339))
		}
		if !until.IsZero() {
			params.Set("until", until.UTC().Format(time.RFC3339))
		}
		if params.Encode() != "" {
			u = u + "&" + params.Encode()
		}

		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			return 0, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		if g.token != "" {
			req.Header.Set("Authorization", "token "+g.token)
		}
		req.Header.Set("User-Agent", "bragdoc-app")

		resp, err := g.client.Do(req)
		if err != nil {
			return 0, err
		}
		defer resp.Body.Close()

		if resp.StatusCode == 404 {
			return 0, nil
		}
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			return 0, fmt.Errorf("github commits status=%d: %s", resp.StatusCode, string(b))
		}

		// If GitHub provided Link header with rel="last", parse last page number.
		link := resp.Header.Get("Link")
		if link != "" {
			if last := parseLastPage(link); last > 0 {
				return last, nil
			}
		}

		// No Link header - decode array and count entries
		var commits []any
		if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
			return 0, err
		}
		return len(commits), nil
	}

	// If an author string is provided, first try using it as a GitHub login
	// (fast, server-side filter). If that doesn't return results, fall back
	// to scanning commits and compare by `author.login` OR commit author name.

	// Try to obtain the authenticated user's `name` (may be empty). This
	// lets us compare commit author names against the user's full name
	// similarly to the Java implementation (`myName.contains(authorName)`).
	var myName string
	if g.token != "" {
		userURL := fmt.Sprintf("%s/user", g.baseURL)
		reqUser, err := http.NewRequest("GET", userURL, nil)
		if err == nil {
			reqUser.Header.Set("Accept", "application/vnd.github+json")
			reqUser.Header.Set("Authorization", "token "+g.token)
			reqUser.Header.Set("User-Agent", "bragdoc-app")
			respUser, err := g.client.Do(reqUser)
			if err == nil {
				defer respUser.Body.Close()
				if respUser.StatusCode == 200 {
					var u struct {
						Name string `json:"name"`
					}
					if err := json.NewDecoder(respUser.Body).Decode(&u); err == nil {
						myName = strings.TrimSpace(u.Name)
					}
				}
			}
		}
	}

	u := fmt.Sprintf("%s/repos/%s/commits?per_page=1", g.baseURL, ownerRepo)
	params := url.Values{}
	params.Set("author", author)
	if !since.IsZero() {
		params.Set("since", since.UTC().Format(time.RFC3339))
	}
	if !until.IsZero() {
		params.Set("until", until.UTC().Format(time.RFC3339))
	}
	if params.Encode() != "" {
		u = u + "&" + params.Encode()
	}

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.token != "" {
		req.Header.Set("Authorization", "token "+g.token)
	}
	req.Header.Set("User-Agent", "bragdoc-app")

	resp, err := g.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return 0, nil
	}
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("github commits status=%d: %s", resp.StatusCode, string(b))
	}

	// If GitHub provided Link header with rel="last", parse last page number.
	link := resp.Header.Get("Link")
	if link != "" {
		if last := parseLastPage(link); last > 0 {
			return last, nil
		}
	}

	// No Link header - decode array and count entries (this may be 0).
	var commits []any
	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		return 0, err
	}
	if len(commits) > 0 {
		return len(commits), nil
	}

	// Fallback: iterate commits (pages) and compare by author.login or commit.author.name.
	perPage := 100
	count := 0
	for page := 1; ; page++ {
		u := fmt.Sprintf("%s/repos/%s/commits?per_page=%d&page=%d", g.baseURL, ownerRepo, perPage, page)
		q := url.Values{}
		if !since.IsZero() {
			q.Set("since", since.UTC().Format(time.RFC3339))
		}
		if !until.IsZero() {
			q.Set("until", until.UTC().Format(time.RFC3339))
		}
		if q.Encode() != "" {
			u = u + "&" + q.Encode()
		}

		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			return 0, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		if g.token != "" {
			req.Header.Set("Authorization", "token "+g.token)
		}
		req.Header.Set("User-Agent", "bragdoc-app")

		resp, err := g.client.Do(req)
		if err != nil {
			return 0, err
		}
		if resp.StatusCode == 404 {
			resp.Body.Close()
			return 0, nil
		}
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return 0, fmt.Errorf("github commits status=%d: %s", resp.StatusCode, string(b))
		}

		var ghCommits []struct {
			Commit struct {
				Author struct {
					Name string `json:"name"`
				} `json:"author"`
			} `json:"commit"`
			Author *struct {
				Login string `json:"login"`
			} `json:"author"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&ghCommits); err != nil {
			resp.Body.Close()
			return 0, err
		}
		resp.Body.Close()

		if len(ghCommits) == 0 {
			break
		}
		for _, c := range ghCommits {
			commitName := strings.TrimSpace(c.Commit.Author.Name)
			loginMatch := c.Author != nil && strings.EqualFold(c.Author.Login, author)
			nameEqualMatch := commitName != "" && (strings.EqualFold(commitName, author) || (myName != "" && strings.EqualFold(commitName, myName)))
			containsMatch := false
			if myName != "" && commitName != "" {
				lm := strings.ToLower(myName)
				cm := strings.ToLower(commitName)
				if strings.Contains(lm, cm) || strings.Contains(cm, lm) {
					containsMatch = true
				}
			}
			if loginMatch || nameEqualMatch || containsMatch {
				count++
			}
		}
		if len(ghCommits) < perPage {
			break
		}
	}
	return count, nil
}

func parseLastPage(link string) int {
	// Link header example: <https://api.github.com/.../commits?page=10>; rel="last", <...>; rel="next"
	parts := strings.Split(link, ",")
	for _, p := range parts {
		if strings.Contains(p, "rel=\"last\"") {
			// extract url between <>
			start := strings.Index(p, "<")
			end := strings.Index(p, ">")
			if start == -1 || end == -1 || end <= start+1 {
				continue
			}
			u := p[start+1 : end]
			// find page= param
			if idx := strings.Index(u, "page="); idx != -1 {
				sub := u[idx+5:]
				// trim after & if present
				if amp := strings.Index(sub, "&"); amp != -1 {
					sub = sub[:amp]
				}
				if v, err := strconv.Atoi(sub); err == nil {
					return v
				}
			}
		}
	}
	return 0
}

// CommitInfo represents a simplified commit payload extracted from GitHub API.
type CommitInfo struct {
	SHA         string    `json:"sha"`
	Message     string    `json:"message"`
	AuthorLogin string    `json:"authorLogin,omitempty"`
	AuthorName  string    `json:"authorName,omitempty"`
	Date        time.Time `json:"date"`
}

// ListCommitMessages returns commits (sha, message, author, date) for the
// given repository. If author is provided, it will try a server-side filter
// first and then fall back to client-side filtering similar to CountCommits.
func (g *GitHubClient) ListCommitMessages(ownerRepo, author string, since, until time.Time) ([]CommitInfo, error) {
	var out []CommitInfo
	perPage := 100

	// try to obtain authenticated user's name for fuzzy matching
	var myName string
	if g.token != "" {
		userURL := fmt.Sprintf("%s/user", g.baseURL)
		reqUser, err := http.NewRequest("GET", userURL, nil)
		if err == nil {
			reqUser.Header.Set("Accept", "application/vnd.github+json")
			reqUser.Header.Set("Authorization", "token "+g.token)
			reqUser.Header.Set("User-Agent", "bragdoc-app")
			respUser, err := g.client.Do(reqUser)
			if err == nil {
				defer respUser.Body.Close()
				if respUser.StatusCode == 200 {
					var u struct {
						Name string `json:"name"`
					}
					if err := json.NewDecoder(respUser.Body).Decode(&u); err == nil {
						myName = strings.TrimSpace(u.Name)
					}
				}
			}
		}
	}

	// If author provided, try server-side filter first
	if author != "" {
		for page := 1; ; page++ {
			u := fmt.Sprintf("%s/repos/%s/commits?per_page=%d&page=%d", g.baseURL, ownerRepo, perPage, page)
			params := url.Values{}
			params.Set("author", author)
			if !since.IsZero() {
				params.Set("since", since.UTC().Format(time.RFC3339))
			}
			if !until.IsZero() {
				params.Set("until", until.UTC().Format(time.RFC3339))
			}
			if params.Encode() != "" {
				u = u + "&" + params.Encode()
			}

			req, err := http.NewRequest("GET", u, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Accept", "application/vnd.github+json")
			if g.token != "" {
				req.Header.Set("Authorization", "token "+g.token)
			}
			req.Header.Set("User-Agent", "bragdoc-app")

			resp, err := g.client.Do(req)
			if err != nil {
				return nil, err
			}
			if resp.StatusCode == 404 {
				resp.Body.Close()
				return nil, nil
			}
			if resp.StatusCode != 200 {
				b, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				return nil, fmt.Errorf("github commits status=%d: %s", resp.StatusCode, string(b))
			}

			var ghCommits []struct {
				SHA    string `json:"sha"`
				Commit struct {
					Message string `json:"message"`
					Author  struct {
						Name string `json:"name"`
						Date string `json:"date"`
					} `json:"author"`
				} `json:"commit"`
				Author *struct {
					Login string `json:"login"`
				} `json:"author"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&ghCommits); err != nil {
				resp.Body.Close()
				return nil, err
			}
			resp.Body.Close()

			if len(ghCommits) == 0 {
				break
			}
			for _, c := range ghCommits {
				var d time.Time
				if t := strings.TrimSpace(c.Commit.Author.Date); t != "" {
					d, _ = time.Parse(time.RFC3339, t)
				}
				ci := CommitInfo{
					SHA:        c.SHA,
					Message:    c.Commit.Message,
					AuthorName: strings.TrimSpace(c.Commit.Author.Name),
					Date:       d,
				}
				if c.Author != nil {
					ci.AuthorLogin = c.Author.Login
				}
				out = append(out, ci)
			}
			if len(ghCommits) < perPage {
				break
			}
		}
		if len(out) > 0 {
			return out, nil
		}
		// else fallthrough to full scan
	}

	// Full scan (no author or server-side filter returned nothing)
	for page := 1; ; page++ {
		u := fmt.Sprintf("%s/repos/%s/commits?per_page=%d&page=%d", g.baseURL, ownerRepo, perPage, page)
		q := url.Values{}
		if !since.IsZero() {
			q.Set("since", since.UTC().Format(time.RFC3339))
		}
		if !until.IsZero() {
			q.Set("until", until.UTC().Format(time.RFC3339))
		}
		if q.Encode() != "" {
			u = u + "&" + q.Encode()
		}

		req, err := http.NewRequest("GET", u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		if g.token != "" {
			req.Header.Set("Authorization", "token "+g.token)
		}
		req.Header.Set("User-Agent", "bragdoc-app")

		resp, err := g.client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == 404 {
			resp.Body.Close()
			return nil, nil
		}
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("github commits status=%d: %s", resp.StatusCode, string(b))
		}

		var ghCommits []struct {
			SHA    string `json:"sha"`
			Commit struct {
				Message string `json:"message"`
				Author  struct {
					Name string `json:"name"`
					Date string `json:"date"`
				} `json:"author"`
			} `json:"commit"`
			Author *struct {
				Login string `json:"login"`
			} `json:"author"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&ghCommits); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		if len(ghCommits) == 0 {
			break
		}
		for _, c := range ghCommits {
			commitName := strings.TrimSpace(c.Commit.Author.Name)
			loginMatch := c.Author != nil && strings.EqualFold(c.Author.Login, author)
			nameEqualMatch := commitName != "" && (strings.EqualFold(commitName, author) || (myName != "" && strings.EqualFold(commitName, myName)))
			containsMatch := false
			if myName != "" && commitName != "" {
				lm := strings.ToLower(myName)
				cm := strings.ToLower(commitName)
				if strings.Contains(lm, cm) || strings.Contains(cm, lm) {
					containsMatch = true
				}
			}
			include := true
			if author != "" {
				include = (loginMatch || nameEqualMatch || containsMatch)
			}
			if include {
				var d time.Time
				if t := strings.TrimSpace(c.Commit.Author.Date); t != "" {
					d, _ = time.Parse(time.RFC3339, t)
				}
				ci := CommitInfo{
					SHA:        c.SHA,
					Message:    c.Commit.Message,
					AuthorName: commitName,
					Date:       d,
				}
				if c.Author != nil {
					ci.AuthorLogin = c.Author.Login
				}
				out = append(out, ci)
			}
		}
		if len(ghCommits) < perPage {
			break
		}
	}
	return out, nil
}
