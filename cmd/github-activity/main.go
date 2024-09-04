package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/Garetonchick/github-user-activity/pkg/github"
)

type EventsDigest struct {
	CommitsPushed       map[string]int
	LastIssueOpenedRepo string
	LastStar            string
}

func MakeEventsDigest(events []github.Event) *EventsDigest {
	var digest EventsDigest
	digest.CommitsPushed = make(map[string]int)

	processPushEvent := func(e *github.Event) error {
		var fields map[string]json.RawMessage
		err := json.Unmarshal(e.Payload, &fields)
		if err != nil {
			return err
		}

		size, ok := fields["size"]
		if !ok {
			return errors.New("no field \"size\" inside payload")
		}
		sz, err := strconv.ParseInt(string(size), 10, 64)
		if err != nil {
			return errors.New("field \"size\" is not an int")
		}
		digest.CommitsPushed[e.Repo.Name] += int(sz)

		return nil
	}

	processIssuesEvent := func(e *github.Event) error {
		if digest.LastIssueOpenedRepo != "" {
			return nil
		}

		var fields map[string]any
		err := json.Unmarshal(e.Payload, &fields)
		if err != nil {
			return err
		}

		action, ok := fields["action"]
		if !ok {
			return errors.New("no field \"action\" inside payload")
		}
		saction, ok := action.(string)
		if !ok {
			return errors.New("field \"action\" is not a string")
		}
		if saction == "opened" {
			digest.LastIssueOpenedRepo = e.Repo.Name
		}
		return nil
	}

	processWatchEvent := func(e *github.Event) {
		if digest.LastStar != "" {
			return
		}
		digest.LastStar = e.Repo.Name
	}

	processEvent := func(e *github.Event) error {
		switch e.Type {
		case "PushEvent":
			return processPushEvent(e)
		case "IssuesEvent":
			return processIssuesEvent(e)
		case "WatchEvent":
			processWatchEvent(e)
		default:
		}
		return nil
	}

	for _, e := range events {
		err := processEvent(&e)
		if err != nil {
			log.Fatal(err)
		}
	}

	return &digest
}

func PrintEventsDigest(digest *EventsDigest) {
	was := false
	for repo, count := range digest.CommitsPushed {
		was = true
		fmt.Printf("Pushed %d commits to %s\n", count, repo)
	}

	if digest.LastIssueOpenedRepo != "" {
		was = true
		fmt.Printf("Opened a new issue in %s\n", digest.LastIssueOpenedRepo)
	}
	if digest.LastStar != "" {
		was = true
		fmt.Printf("Starred %s\n", digest.LastStar)
	}

	if !was {
		fmt.Println("User has no activity")
	}
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Expected username")
	}
	username := os.Args[1]
	githubClient := github.NewClient(http.DefaultClient)

	events, err := githubClient.GetUserEvents(context.Background(), username)
	if err != nil {
		log.Fatal(err)
	}

	digest := MakeEventsDigest(events)
	PrintEventsDigest(digest)
}
