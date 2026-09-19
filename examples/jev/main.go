package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mhrlife/goai-kit/jev"
)

func main() {
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		log.Fatal("set OPENROUTER_API_KEY")
	}
	client := jev.New(key)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	response, err := client.Decide(ctx, "My payment has failed three times today.", jev.Questions{
		"urgent": jev.NoulQuestion{Instructions: "Does this need urgent attention?"},
		"team": jev.ChoiceQuestion{
			Instructions: "Which team should handle this?",
			Criteria:     jev.Options{"billing": "Payments and invoices", "technical": "Bugs and outages", "other": ""},
		},
		"frustration": jev.ScoreQuestion{
			Instructions: "How frustrated is the customer?",
			Criteria:     []string{"Calm", "Frustrated", "Very angry"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	for id, answer := range response.Answers {
		switch answer := answer.(type) {
		case jev.NoulAnswer:
			fmt.Printf("%s: probability of yes = %.2f\n", id, answer.Noul)
		case jev.ChoiceAnswer:
			fmt.Printf("%s: %s (confidence %.2f)\n", id, answer.Choice, answer.Confidence)
		case jev.ScoreAnswer:
			fmt.Printf("%s: %.2f (confidence %.2f)\n", id, answer.Score, answer.Confidence)
		}
	}
}
