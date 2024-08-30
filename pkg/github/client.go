package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const ENDPOINT_BASE = "https://api.github.com/users/"

type Actor struct {
	ID           uint64 `json:"id"`
	Login        string `json:"login"`
	DisplayLogin string `json:"display_login"`
	GravatarID   string `json:"gravatar_id"`
	URL          string `json:"url"`
	AvatarURL    string `json:"avatar_url"`
}

type Repo struct {
	ID   uint64 `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type Organisation struct {
	ID         uint64 `json:"id"`
	Login      string `json:"login"`
	GravatarID string `json:"gravatar_id"`
	URL        string `json:"url"`
	AvatarURL  string `json:"avatar_url"`
}

type Event struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Actor        Actor           `json:"actor"`
	Repo         Repo            `json:"repo"`
	Payload      json.RawMessage `json:"payload"`
	Public       bool            `json:"public"`
	CreatedAt    string          `json:"created_at"`
	Organisation *Organisation   `json:"org,omitempty"`
}

type Client struct {
	client           *http.Client
	lastPollTime     time.Time
	lastPollInterval time.Duration
}

type GithubResponseHeaders struct {
	XPollInterval       time.Duration
	XRatelimitLimit     int
	XRatelimitRemaining int
	XRatelimitUsed      int
	XRatelimitReset     time.Time
	XRatelimitResource  string
}

func NewClient(client *http.Client) *Client {
	return &Client{client: client}
}

func (c *Client) NeedsToWait() time.Duration {
	return c.lastPollInterval - time.Since(c.lastPollTime)
}

func (c *Client) Get(ctx context.Context, endpointURL string) (*http.Response, error) {
	c.waitPollInterval()
	c.lastPollTime = time.Now()
	c.lastPollInterval = time.Second

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	headers, err := parseHTTPHeaders(&resp.Header)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}

	c.lastPollInterval = headers.XPollInterval

	if headers.XRatelimitRemaining == 0 {
		c.lastPollInterval = time.Until(headers.XRatelimitReset)
	}

	return resp, nil

}

// Unmarshals json inside struct pointed by v
func (c *Client) GetJSON(ctx context.Context, endpointURL string, v any) error {
	resp, err := c.Get(ctx, endpointURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	rawJSON, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(rawJSON, v)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) GetUserEvents(ctx context.Context, user string) ([]Event, error) {
	eventsURL, err := c.buildUserEventsURL(user)
	if err != nil {
		return nil, err
	}

	var events []Event
	err = c.GetJSON(ctx, eventsURL, &events)
	if err != nil {
		return nil, err
	}

	return events, nil
}

func (c *Client) waitPollInterval() {
	time.Sleep(c.NeedsToWait())
}

func parseIntHeader(s string) (int, error) {
	if s == "" {
		return 0, nil
	}

	val, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}

	return val, nil
}

func parseTimeHeader(s string) (time.Time, error) {
	if s == "" {
		return time.Now(), nil
	}
	nSeconds, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	if nSeconds < 0 {
		return time.Time{}, fmt.Errorf("failed to convert %q seconds to unix time", s)
	}

	return time.Unix(nSeconds, 0), nil
}

func parseDurationHeader(s string) (time.Duration, error) {
	nSeconds, err := parseIntHeader(s)
	if err != nil {
		return time.Duration(0), err
	}
	return time.Duration(nSeconds) * time.Second, nil
}

func field2HeaderName(name string) string {
	if len(name) == 0 {
		panic("empty field name")
	}

	var sb strings.Builder
	sb.Grow(len(name) + 3)

	for i := 0; i+1 < len(name); i++ {
		sb.WriteByte(name[i])
		if unicode.IsUpper(rune(name[i+1])) {
			sb.WriteByte('-')
		}
	}
	sb.WriteByte(name[len(name)-1])
	return sb.String()
}

func parseHTTPHeaders(headers *http.Header) (*GithubResponseHeaders, error) {
	var parsedHeaders GithubResponseHeaders
	pVal := reflect.ValueOf(&parsedHeaders)
	val := pVal.Elem()
	valType := val.Type()

	for i := 0; i < val.NumField(); i++ {
		fVal := val.Field(i)
		structField := valType.Field(i)
		fName := structField.Name

		headerName := structField.Tag.Get("header")
		if len(headerName) == 0 {
			headerName = field2HeaderName(fName)
		}

		headerRawVal := headers.Get(headerName)
		fmt.Printf("header: %q\n", headerName)

		switch fVal.Interface().(type) {
		case int:
			value, err := parseIntHeader(headerRawVal)
			if err != nil {
				return nil, err
			}
			fVal.SetInt(int64(value))
		case string:
			fVal.SetString(headerRawVal)
		case time.Duration:
			value, err := parseDurationHeader(headerRawVal)
			if err != nil {
				return nil, err
			}
			fVal.Set(reflect.ValueOf(value))
		case time.Time:
			value, err := parseTimeHeader(headerRawVal)
			if err != nil {
				return nil, err
			}
			fVal.Set(reflect.ValueOf(value))
		default:
			return nil, fmt.Errorf("unsupported header type \"%T\"", fVal.Interface())
		}
	}

	return &parsedHeaders, nil
}

func (c *Client) buildUserEventsURL(user string) (string, error) {
	eventsURL, err := url.JoinPath(ENDPOINT_BASE, user, "events")
	if err != nil {
		return "", err
	}
	return eventsURL, nil
}
