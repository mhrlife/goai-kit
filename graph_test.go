package goaikit

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"log/slog"
	"math/rand"
	"os"
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

func TestGraphRetry(t *testing.T) {
	// 1. Define the data structure (Context) the graph will use
	type RetryGraphContext struct {
		RetryCount int
	}

	goaiClient := NewClient(
		WithDefaultModel("google/gemini-2.5-flash-preview-05-20"),
		WithAPIKey(os.Getenv("OPENROUTER_API_KEY")),
		WithBaseURL(os.Getenv("OPENROUTER_API_BASE")),
		WithLogLevel(slog.LevelDebug),
	)

	// 3. Define nodes
	const (
		StageRetryNode = "retry_node"
	)

	retryNode := Node[RetryGraphContext]{
		Name: StageRetryNode,
		Runner: func(ctx context.Context, arg NodeArg[RetryGraphContext]) (RetryGraphContext, string, error) {
			arg.Context.RetryCount++
			t.Logf("Retry count: %d", arg.Context.RetryCount)
			if arg.Context.RetryCount < 3 {
				return arg.Context, GraphRetry, nil
			}
			return arg.Context, GraphExit, nil
		},
	}

	// 4. Create the graph
	graph, err := NewGraph("retry_test", retryNode)
	require.NoError(t, err)

	// 5. Run the graph
	initialContext := RetryGraphContext{RetryCount: 0}
	finalContext, err := graph.Run(context.Background(), goaiClient, initialContext)
	require.NoError(t, err)

	// 6. Assert the result
	require.NotNil(t, finalContext)
	require.Equal(t, 3, finalContext.RetryCount)
}

func TestGraphExecutionWithAICallNode(t *testing.T) {
	type AIGraphContext struct {
		InitialIdea string
		RefinedIdea string
	}

	goaiClient := NewClient(
		WithDefaultModel("google/gemini-2.5-flash-preview-05-20"),
		WithAPIKey(os.Getenv("OPENROUTER_API_KEY")),
		WithBaseURL(os.Getenv("OPENROUTER_API_BASE")),
		WithLogLevel(slog.LevelDebug),
	)

	type IdeaOutput struct {
		Idea string `json:"idea" jsonschema:"description=A short, innovative business idea."`
	}

	generateIdeaNode := NewAICallNode(AICallNode[AIGraphContext, IdeaOutput]{
		Name: "generate_idea",
		PromptGenerator: func(graphContext AIGraphContext) string {
			return "Suggest a new business idea for a tech startup."
		},
		Callback: func(ctx context.Context, arg NodeArg[AIGraphContext], aiOutput *IdeaOutput) (AIGraphContext, string, error) {
			arg.Context.InitialIdea = aiOutput.Idea
			t.Logf("Generated Idea: %s", arg.Context.InitialIdea)
			return arg.Context, "refine_idea", nil
		},
	})

	type RefinedIdeaOutput struct {
		RefinedIdea string `json:"refined_idea" jsonschema:"description=A refined version of the business idea, making it more specific."`
	}

	refineIdeaNode := NewAICallNode(AICallNode[AIGraphContext, RefinedIdeaOutput]{
		Name: "refine_idea",
		PromptGenerator: func(graphContext AIGraphContext) string {
			return fmt.Sprintf("Take this business idea and make it more specific and actionable: '%s'", graphContext.InitialIdea)
		},
		Callback: func(ctx context.Context, arg NodeArg[AIGraphContext], aiOutput *RefinedIdeaOutput) (AIGraphContext, string, error) {
			arg.Context.RefinedIdea = aiOutput.RefinedIdea
			t.Logf("Refined Idea: %s", arg.Context.RefinedIdea)
			return arg.Context, GraphExit, nil
		},
	})

	graph, err := NewGraph("ai_idea_generator",
		generateIdeaNode,
		refineIdeaNode,
	)
	require.NoError(t, err)

	finalContext, err := graph.Run(context.Background(), goaiClient, AIGraphContext{})
	require.NoError(t, err)
	require.NotEmpty(t, finalContext.InitialIdea, "The initial idea should not be empty")
	require.NotEmpty(t, finalContext.RefinedIdea, "The refined idea should not be empty")
}
