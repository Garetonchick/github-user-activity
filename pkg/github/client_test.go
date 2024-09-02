package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"testing"
	"time"
)

var basicUserEvents map[string][]Event
var defaultServerSettings serverSettings

type serverSettings struct {
	UserEvents                 map[string][]Event
	Ratelimit                  bool
	RatelimitWindowSize        int
	RatelimitRequestsPerWindow int
}

func loadJSONFromFile(path string, v any) error {
	rawJSON, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return json.Unmarshal(rawJSON, v)
}

func TestMain(m *testing.M) {
	err := loadJSONFromFile("testdata/basic_user_events.json", &basicUserEvents)
	if err != nil {
		panic(err)
	}
	defaultServerSettings = serverSettings{
		UserEvents: basicUserEvents,
		Ratelimit:  false,
	}

	os.Exit(m.Run())
}

func newTestServer(t *testing.T, settings *serverSettings) *httptest.Server {
	rexp, err := regexp.Compile("^/users/([^/]+)/events$")
	if err != nil {
		panic(err)
	}
	var windowRequests int
	ratelimitWindowSize := time.Duration(settings.RatelimitWindowSize) * time.Second
	windowResetTime := time.Now().Add(ratelimitWindowSize)
	svr := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET method but got %q", r.Method)
		}

		matches := rexp.FindStringSubmatch(r.URL.Path)
		if matches == nil {
			t.Fatalf("wrong endpoint path %q", r.URL.Path)
		}

		currentTime := time.Now()

		if windowResetTime.Before(currentTime) {
			windowRequests = 0
			windowResetTime = windowResetTime.Add(ratelimitWindowSize)
			if windowResetTime.Before(currentTime) {
				windowResetTime = currentTime.Add(ratelimitWindowSize)
			}
		}

		windowRequests += 1

		if settings.Ratelimit && windowRequests > settings.RatelimitWindowSize {
			if rand.Uint64()&1 == 0 {
				rw.WriteHeader(http.StatusForbidden)
			} else {
				rw.WriteHeader(http.StatusTooManyRequests)
			}
			rw.Header().Add("X-Ratelimit-Limit", fmt.Sprintf("%d", settings.RatelimitRequestsPerWindow))
			rw.Header().Add("X-Ratelimit-Remaining", "0")
			rw.Header().Add("X-Ratelimit-Used", rw.Header().Get("X-Ratelimit-Limit"))
			rw.Header().Add("X-Ratelimit-Reset", fmt.Sprintf("%d", windowResetTime.Unix()))
			return
		}

		username := matches[1]
		events, ok := settings.UserEvents[username]
		if !ok {
			rw.WriteHeader(http.StatusNotFound)
			return
		}

		if settings.Ratelimit {
			remaining := settings.RatelimitRequestsPerWindow - windowRequests
			if remaining > 0 {
				pollInterval := int64(windowResetTime.Sub(currentTime).Seconds()/float64(remaining)) + 1
				rw.Header().Add("X-Poll-Interval", fmt.Sprintf("%d", pollInterval))
			}
			rw.Header().Add("X-Ratelimit-Limit", fmt.Sprintf("%d", settings.RatelimitRequestsPerWindow))
			rw.Header().Add("X-Ratelimit-Remaining", fmt.Sprintf("%d", remaining))
			rw.Header().Add("X-Ratelimit-Used", fmt.Sprintf("%d", windowRequests))
			rw.Header().Add("X-Ratelimit-Reset", fmt.Sprintf("%d", windowResetTime.Unix()))
		}

		jsonEvents, err := json.Marshal(events)
		if err != nil {
			panic("bad events")
		}

		_, err = rw.Write(jsonEvents)
		if err != nil {
			panic(err)
		}
	}))
	endpointBase = svr.URL
	return svr
}

func TestGetUserEvents(t *testing.T) {
	svr := newTestServer(t, &defaultServerSettings)
	defer svr.Close()

	c := NewClient(http.DefaultClient)
	user := "garetonchick"
	got, err := c.GetUserEvents(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	expected := basicUserEvents[user]
	if reflect.DeepEqual(got, expected) {
		t.Fatalf("EXPECTED:\n%v\n\n\n\nGOT:\n%v\n", expected, got)
	}
}

func TestUnknownUser(t *testing.T) {
	svr := newTestServer(t, &defaultServerSettings)
	defer svr.Close()

	c := NewClient(http.DefaultClient)
	user := "aboba"
	_, err := c.GetUserEvents(context.Background(), user)
	if err == nil {
		t.Fatalf("Expected error")
	}
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("Expected ErrUserNotFound error, but got %q error instead", err)
	}
}

func TestRatelimit(t *testing.T) {
	settings := serverSettings{
		UserEvents:                 basicUserEvents,
		Ratelimit:                  true,
		RatelimitWindowSize:        4,
		RatelimitRequestsPerWindow: 2,
	}
	svr := newTestServer(t, &settings)
	defer svr.Close()

	c := NewClient(http.DefaultClient)

	startTime := time.Now()

	nWindows := 2

	// it misses window one time, so it has to wait extra
	for range settings.RatelimitRequestsPerWindow * nWindows {
		user := "garetonchick"
		_, err := c.GetUserEvents(context.Background(), user)
		if err != nil {
			t.Fatal(err)
		}
	}

	got := time.Since(startTime)
	expected := time.Duration(settings.RatelimitWindowSize*(nWindows+1)) * time.Second
	if (got - expected).Abs() > time.Millisecond*100 {
		t.Fatalf("Expected %dms execution, but got %dms execution", expected.Milliseconds(), got.Milliseconds())
	}
}

func TestContextCancelling(t *testing.T) {

}

func TestParseHeaders(t *testing.T) {

}

func TestField2HeaderName(t *testing.T) {

}
