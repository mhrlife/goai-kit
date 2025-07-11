package goaikit

import (
	"context"
	"fmt"
	"github.com/henomis/langfuse-go/model"
	"github.com/pkg/errors"
)

var (
	GraphExit  string = ""
	GraphRetry string = "__built_in__retry"
)

type Graph[Context any] struct {
	name       string
	nodes      map[string]Node[Context]
	entrypoint string
}

func NewGraph[Context any](name string, nodes ...Node[Context]) (*Graph[Context], error) {
	if len(nodes) == 0 {
		return nil, errors.New("graph must have at least one node")
	}

	nodeMap := make(map[string]Node[Context])
	for _, node := range nodes {
		if _, exists := nodeMap[node.Name]; exists {
			return nil, fmt.Errorf("duplicate node name found: %s", node.Name)
		}

		nodeMap[node.Name] = node
	}

	return &Graph[Context]{
		name:       name,
		nodes:      nodeMap,
		entrypoint: nodes[0].Name,
	}, nil
}

func (g *Graph[Context]) Run(ctx context.Context, client *Client, initialContext Context) (*Context, error) {
	return WithTrace[Context](
		ctx,
		client,
		&model.Trace{
			Name: "graph_" + g.name,
		},
		func(ctx context.Context) (*Context, error) {
			return g.run(ctx, client, initialContext)
		},
	)
}

func (g *Graph[Context]) run(ctx context.Context, client *Client, initialContext Context) (*Context, error) {
	currentContext := initialContext
	currentNodeName := g.entrypoint

	for currentNodeName != GraphExit {
		node, ok := g.nodes[currentNodeName]
		if !ok {
			return nil, fmt.Errorf("node '%s' not found in graph", currentNodeName)
		}

		nodeArg := NodeArg[Context]{
			Context:  currentContext,
			Client:   client,
			Metadata: make(map[string]any),
		}

		var err error
		var nextNodeName string
		currentContext, nextNodeName, err = node.Runner(ctx, nodeArg)
		if err != nil {
			client.logger.Error("Node execution failed",
				"node_name", node.Name,
				"error", err,
			)

			return nil, errors.Wrapf(err, "failed to run node %s", node.Name)
		}

		if nextNodeName == GraphRetry {
			continue
		}

		currentNodeName = nextNodeName
	}

	return &currentContext, nil
}
