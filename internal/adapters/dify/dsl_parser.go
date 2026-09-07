// Package dify imports Dify applications and workflows.
package dify

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// DSL is the subset of a Dify application export that this importer understands.
//
// The shape deliberately follows a real Dify DSL export (app / kind / version /
// workflow.graph) rather than an invented schema, so a DSL a customer exports from
// their own Dify has a genuine chance of parsing. Fields Dify emits that carry no
// capability meaning are ignored, and unknown node types are skipped with a warning
// instead of failing the import.
type DSL struct {
	App      App      `yaml:"app"`
	Kind     string   `yaml:"kind"`
	Version  string   `yaml:"version"`
	Workflow Workflow `yaml:"workflow"`
}

// App is the Dify application header.
type App struct {
	Name        string `yaml:"name"`
	Mode        string `yaml:"mode"`
	Icon        string `yaml:"icon"`
	Description string `yaml:"description"`
}

// Workflow wraps the node graph.
type Workflow struct {
	Graph Graph `yaml:"graph"`
}

// Graph is the node/edge structure Dify renders on its canvas.
type Graph struct {
	Nodes []Node `yaml:"nodes"`
	Edges []Edge `yaml:"edges"`
}

// Node is one canvas node. Dify nests everything meaningful under `data`.
type Node struct {
	ID   string   `yaml:"id"`
	Type string   `yaml:"type"`
	Data NodeData `yaml:"data"`
}

// NodeData covers the node kinds that carry capability meaning.
type NodeData struct {
	Type  string `yaml:"type"`
	Title string `yaml:"title"`
	Desc  string `yaml:"desc"`

	// tool nodes
	ProviderID   string         `yaml:"provider_id"`
	ProviderName string         `yaml:"provider_name"`
	ProviderType string         `yaml:"provider_type"`
	ToolName     string         `yaml:"tool_name"`
	ToolParams   map[string]any `yaml:"tool_parameters"`

	// knowledge-retrieval nodes
	DatasetIDs []string `yaml:"dataset_ids"`

	// llm nodes
	Model          map[string]any  `yaml:"model"`
	PromptTemplate []PromptMessage `yaml:"prompt_template"`

	// start nodes
	Variables []Variable `yaml:"variables"`

	// answer / end nodes
	Answer  string     `yaml:"answer"`
	Outputs []Variable `yaml:"outputs"`
}

// PromptMessage is one entry of an llm node's prompt template.
type PromptMessage struct {
	Role string `yaml:"role"`
	Text string `yaml:"text"`
}

// Variable is a start-node input or an end-node output declaration.
type Variable struct {
	Variable string `yaml:"variable"`
	Label    string `yaml:"label"`
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
}

// Edge connects two nodes.
type Edge struct {
	ID     string `yaml:"id"`
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

// Known Dify node types.
const (
	NodeStart              = "start"
	NodeLLM                = "llm"
	NodeTool               = "tool"
	NodeKnowledgeRetrieval = "knowledge-retrieval"
	NodeAnswer             = "answer"
	NodeEnd                = "end"
)

// ParseResult is a parsed DSL plus anything that was skipped.
type ParseResult struct {
	DSL      DSL
	Warnings []string
}

// Parse reads a Dify DSL document.
func Parse(payload []byte) (ParseResult, error) {
	var result ParseResult
	if len(strings.TrimSpace(string(payload))) == 0 {
		return result, fmt.Errorf("empty dsl document")
	}
	if err := yaml.Unmarshal(payload, &result.DSL); err != nil {
		return result, fmt.Errorf("parse dify dsl: %w", err)
	}
	if result.DSL.App.Name == "" {
		return result, fmt.Errorf("dify dsl has no app.name; is this a Dify application export?")
	}

	for _, node := range result.DSL.Graph().Nodes {
		switch node.Data.Type {
		case NodeStart, NodeLLM, NodeTool, NodeKnowledgeRetrieval, NodeAnswer, NodeEnd:
		case "":
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("node %q has no data.type and was skipped", node.ID))
		default:
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("node %q has unsupported type %q and was skipped", node.ID, node.Data.Type))
		}
	}
	return result, nil
}

// Graph returns the workflow graph.
func (d DSL) Graph() Graph { return d.Workflow.Graph }

// IsAgentMode reports whether the app is a conversational agent rather than a bare
// workflow. Dify uses chat, agent-chat and advanced-chat for the former.
func (d DSL) IsAgentMode() bool {
	switch strings.ToLower(strings.TrimSpace(d.App.Mode)) {
	case "chat", "agent-chat", "advanced-chat", "completion":
		return true
	default:
		return false
	}
}

// NodesOfType returns every node of a given data.type, in document order.
func (g Graph) NodesOfType(nodeType string) []Node {
	var out []Node
	for _, node := range g.Nodes {
		if node.Data.Type == nodeType {
			out = append(out, node)
		}
	}
	return out
}

// OrderedSteps walks the graph from its start node and returns the titles of the nodes
// that represent business steps, in execution order. It falls back to document order if
// the edges do not form a single path.
func (g Graph) OrderedSteps() []string {
	next := make(map[string]string, len(g.Edges))
	for _, edge := range g.Edges {
		if _, seen := next[edge.Source]; !seen {
			next[edge.Source] = edge.Target
		}
	}
	byID := make(map[string]Node, len(g.Nodes))
	for _, node := range g.Nodes {
		byID[node.ID] = node
	}

	var start string
	for _, node := range g.Nodes {
		if node.Data.Type == NodeStart {
			start = node.ID
			break
		}
	}

	var (
		steps   []string
		visited = map[string]bool{}
	)
	for id := start; id != "" && !visited[id]; id = next[id] {
		visited[id] = true
		node, ok := byID[id]
		if !ok {
			break
		}
		if title := node.StepTitle(); title != "" {
			steps = append(steps, title)
		}
	}
	if len(steps) > 0 {
		return steps
	}
	for _, node := range g.Nodes {
		if title := node.StepTitle(); title != "" {
			steps = append(steps, title)
		}
	}
	return steps
}

// StepTitle returns a workflow step label for nodes that do real work. Start and answer
// nodes are plumbing and produce no step.
func (n Node) StepTitle() string {
	switch n.Data.Type {
	case NodeLLM, NodeTool, NodeKnowledgeRetrieval:
		if n.Data.Title != "" {
			return n.Data.Title
		}
		return n.ID
	default:
		return ""
	}
}

// SystemPrompt concatenates an llm node's prompt template into a single document.
func (n Node) SystemPrompt() string {
	var b strings.Builder
	for _, msg := range n.Data.PromptTemplate {
		if strings.TrimSpace(msg.Text) == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		if msg.Role != "" {
			b.WriteString("[" + msg.Role + "]\n")
		}
		b.WriteString(strings.TrimSpace(msg.Text))
	}
	return b.String()
}
