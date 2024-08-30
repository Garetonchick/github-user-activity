package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/Garetonchick/github-user-activity/pkg/github"
)

func PrintUserEvents(c *github.Client, user string) {
	fmt.Printf("Get events for %q\n", user)
	events, err := c.GetUserEvents(context.TODO(), user)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Got %d events\n", len(events))
	fmt.Printf("Needs to wait %v for next poll\n", c.NeedsToWait())

	n_print := min(len(events), 2)
	fmt.Printf("Printing first %d events\n", n_print)

	for i := 0; i < n_print; i++ {
		events[i].Payload = []byte("removed")
		fmt.Println(events[i])
	}
}

func main() {
	client := http.Client{}
	githubClient := github.NewClient(&client)
	PrintUserEvents(githubClient, "garetonchick")
}
