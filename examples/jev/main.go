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
	client := jev.New(jev.NewOpenRouterClientConfig(key, jev.OpenRouterModel))
	if err := run(client); err != nil {
		log.Fatal(err)
	}
}

// run keeps the work in a function that returns, so the deferred cancel and any
// other cleanup still happen on the error path.
func run(client *jev.Client) error {
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
		return err
	}
	// Each answer is read back as the type the question asked for. A mismatch or a
	// missing id is an error here, not a panic somewhere later.
	urgent, err := response.Answer[jev.NoulAnswer]("urgent")
	if err != nil {
		return err
	}
	team, err := response.Answer[jev.ChoiceAnswer]("team")
	if err != nil {
		return err
	}
	frustration, err := response.Answer[jev.ScoreAnswer]("frustration")
	if err != nil {
		return err
	}

	// Every answer prints itself, so the fields are there when you want them and
	// out of the way when you do not.
	fmt.Println("urgent:     ", urgent)
	fmt.Println("team:       ", team)
	fmt.Println("frustration:", frustration)

	// The whole set prints too, one per line and sorted by question id.
	fmt.Printf("\nall answers:\n%v\n", response.Answers)
	return nil
}
