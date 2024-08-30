package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"testing"
)

var basicUserEvents map[string][]Event

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

	os.Exit(m.Run())
}

func newTestServer(t *testing.T, userEvents map[string][]Event) *httptest.Server {
	rexp, err := regexp.Compile("^/users/([^/]+)/events$")
	if err != nil {
		panic(err)
	}
	svr := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET method but got %q", r.Method)
		}

		matches := rexp.FindStringSubmatch(r.URL.Path)
		if matches == nil {
			t.Fatalf("wrong endpoint path %q", r.URL.Path)
		}

		username := matches[1]
		events, ok := userEvents[username]
		if !ok {
			panic("not implemented")
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
	svr := newTestServer(t, basicUserEvents)
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

func TestNeedsToWait(t *testing.T) {

}

func TestContextCancelling(t *testing.T) {

}

func TestParseHeaders(t *testing.T) {

}

func TestField2HeaderName(t *testing.T) {

}
