package goaikit

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"log/slog"
	"math/rand"
	"testing"
)

// This is the new test for the graph
func TestGraphExecution(t *testing.T) {
	// 1. Define the data structure (Context) the graph will use
	type NumberGraphContext struct {
		CurrentNumber int
		TextualNumber string
	}

	// 2. Setup the client, just like in other tests
	goaiClient := NewClient(
		WithDefaultModel("gpt-4o-mini"),
		WithLogLevel(slog.LevelDebug),
	)

	// 3. Define the nodes for your graph logic

	// Node 1: Suggests a random number
	suggestNumberNode := Node[NumberGraphContext]{
		Name: "suggest_number",
		Runner: func(ctx context.Context, arg NodeArg[NumberGraphContext]) (NumberGraphContext, string, error) {
			arg.Context.CurrentNumber = rand.Intn(10) + 1 // Random number 1-10
			t.Logf("Suggested number: %d", arg.Context.CurrentNumber)
			return arg.Context, "check_odd_even", nil // Go to the next node
		},
	}

	// Node 2: Checks if the number is odd or even
	checkOddEvenNode := Node[NumberGraphContext]{
		Name: "check_odd_even",
		Runner: func(ctx context.Context, arg NodeArg[NumberGraphContext]) (NumberGraphContext, string, error) {
			if arg.Context.CurrentNumber%2 != 0 {
				t.Logf("Number is odd, trying again...")
				return arg.Context, "suggest_number", nil // It's odd, loop back
			}
			t.Logf("Number is even, continuing...")
			return arg.Context, "convert_to_text", nil // It's even, move on
		},
	}

	// Node 3: Converts the number to text locally
	convertToTextNode := Node[NumberGraphContext]{
		Name: "convert_to_text",
		Runner: func(ctx context.Context, arg NodeArg[NumberGraphContext]) (NumberGraphContext, string, error) {
			newContext := arg.Context

			// Simple local conversion instead of AI call
			numberMap := map[int]string{
				2:  "two",
				4:  "four",
				6:  "six",
				8:  "eight",
				10: "ten",
			}

			text, ok := numberMap[newContext.CurrentNumber]
			if !ok {
				// This should not happen if the previous node works correctly (only even numbers 2-10)
				return newContext, "", fmt.Errorf("unexpected even number: %d", newContext.CurrentNumber)
			}

			newContext.TextualNumber = text
			t.Logf("Converted to text: %s", newContext.TextualNumber)
			return newContext, "print_result", nil // Go to the final node
		},
	}

	// Node 4: Prints the final result
	printResultNode := Node[NumberGraphContext]{
		Name: "print_result",
		Runner: func(ctx context.Context, arg NodeArg[NumberGraphContext]) (NumberGraphContext, string, error) {
			t.Logf("The final number is: %s", arg.Context.TextualNumber)
			return arg.Context, "", nil // Stop the graph
		},
	}

	// 4. Create the graph with all the nodes
	graph, err := NewGraph("number_game",
		suggestNumberNode,
		checkOddEvenNode,
		convertToTextNode,
		printResultNode,
	)
	require.NoError(t, err)

	// 5. Run the graph and check the result
	finalContext, err := graph.Run(context.Background(), goaiClient, NumberGraphContext{})
	require.NoError(t, err)
	require.NotEmpty(t, finalContext.TextualNumber, "The final textual number should not be empty")
}
