package direct

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
)

// maxAuthzBody bounds what checkLog reads of an authorization; real ones are
// a few hundred bytes.
const maxAuthzBody = 1 << 20

// checkLog keeps the reason Let's Encrypt gave for the last failed domain
// check. autocert answers a failed check with a new order and returns only
// "no viable challenge type found", which tells an admin nothing.
type checkLog struct {
	base http.RoundTripper

	mu     sync.Mutex
	reason string
}

func (c *checkLog) RoundTrip(r *http.Request) (*http.Response, error) {
	res, err := c.base.RoundTrip(r)
	if err != nil || !strings.Contains(r.URL.Path, "/authz") {
		return res, err
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxAuthzBody))
	_ = res.Body.Close()
	if err != nil {
		return nil, err
	}
	res.Body = io.NopCloser(bytes.NewReader(body))
	if reason := failedCheck(body); reason != "" {
		c.mu.Lock()
		c.reason = reason
		c.mu.Unlock()
	}
	return res, nil
}

func (c *checkLog) last() string {
	if c == nil {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reason
}

// failedCheck is the reason in an invalid ACME authorization, or "".
func failedCheck(authz []byte) string {
	var a struct {
		Status     string `json:"status"`
		Challenges []struct {
			Error *struct {
				Detail string `json:"detail"`
			} `json:"error"`
		} `json:"challenges"`
	}
	if json.Unmarshal(authz, &a) != nil || a.Status != "invalid" {
		return ""
	}
	for _, ch := range a.Challenges {
		if ch.Error != nil && ch.Error.Detail != "" {
			return ch.Error.Detail
		}
	}
	return ""
}

// checkError is a failed domain check, read as Let's Encrypt's reason; the
// autocert error under it adds nothing for an admin.
type checkError struct {
	reason string
	err    error
}

func (e *checkError) Error() string { return e.reason }
func (e *checkError) Unwrap() error { return e.err }
