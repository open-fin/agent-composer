// Command agentctl drives the composition engine from the terminal.
//
// It exists so the demo can be reset and replayed without clicking through the UI:
// `agentctl seed` performs the entire import-and-register walkthrough in one call, and
// `agentctl draft` prints the business agent YAML the presentation ends on.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/open-fin/agent-composer/internal/domain"
	"github.com/open-fin/agent-composer/internal/engine"
)

const usage = `agentctl - drive the agent-composer engine

Usage:
  agentctl [flags] <command> [arguments]

Commands:
  demo                     seed, recommend and print the draft in one run
  seed                     import every bundled example and register the candidates
  import dify <file>       import a Dify DSL export
  import manual <file>     import a manual capability YAML document
  import mcp               import the mock MCP tool gateway inventory
  import data              import the mock knowledge/data catalog
  candidates               list capability candidates
  capabilities             list registered capabilities
  recommend <goal>         recommend a composition for a business goal
  draft <goal>             recommend, generate a draft, and print its YAML

Flags:
  -server URL   composer-server to drive (default http://localhost:8088)
  -local        run the engine in process against an in-memory store instead
                (state lives only for this process, so use the demo command
                 rather than seed followed by a separate draft)
  -dir PATH     example directory for seed (default examples)
  -name NAME    draft name for the draft command (default "RM Campaign Agent")
  -harness H    dify_workflow | jiuwen_swarm | http_agent | manual (default jiuwen_swarm)
  -type T       filter candidates/capabilities by capability type
  -status S     filter candidates by status
`

func main() {
	var (
		server  = flag.String("server", "http://localhost:8088", "composer-server base URL")
		local   = flag.Bool("local", false, "run in process against an in-memory store")
		dir     = flag.String("dir", "examples", "example directory")
		name    = flag.String("name", "RM Campaign Agent", "draft name")
		harness = flag.String("harness", string(domain.HarnessJiuwenSwarm), "target harness type")
		capType = flag.String("type", "", "filter by capability type")
		status  = flag.String("status", "", "filter candidates by status")
	)
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	var client composer
	if *local {
		client = newLocalComposer()
	} else {
		client = newHTTPComposer(*server)
	}

	opts := options{
		dir: *dir, name: *name, harness: domain.HarnessType(*harness),
		capType: *capType, status: *status,
	}
	if err := dispatch(context.Background(), client, args, opts); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

type options struct {
	dir     string
	name    string
	harness domain.HarnessType
	capType string
	status  string
}

func dispatch(ctx context.Context, client composer, args []string, opts options) error {
	switch args[0] {
	case "demo":
		return demo(ctx, client, opts)
	case "seed":
		return seed(ctx, client, opts)
	case "import":
		return runImport(ctx, client, args[1:])
	case "candidates":
		return listCandidates(ctx, client, opts)
	case "capabilities":
		return listCapabilities(ctx, client, opts)
	case "recommend":
		if len(args) < 2 {
			return fmt.Errorf("recommend needs a goal")
		}
		plan, err := client.Recommend(ctx, goalFrom(strings.Join(args[1:], " "), opts))
		if err != nil {
			return err
		}
		printPlan(plan)
		return nil
	case "draft":
		if len(args) < 2 {
			return fmt.Errorf("draft needs a goal")
		}
		return draftCommand(ctx, client, strings.Join(args[1:], " "), opts)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func goalFrom(text string, opts options) domain.BusinessGoal {
	harness := opts.harness
	if !harness.Valid() {
		harness = domain.HarnessJiuwenSwarm
	}
	return domain.BusinessGoal{Name: opts.name, Goal: text, HarnessType: harness}
}

// demo runs the entire walkthrough in a single process. With -local the store lives
// only as long as the command, so seeding and drafting have to happen together.
func demo(ctx context.Context, client composer, opts options) error {
	if err := seed(ctx, client, opts); err != nil {
		return err
	}
	fmt.Println()
	return draftCommand(ctx, client, defaultDemoGoal, opts)
}

// defaultDemoGoal is the business goal the README walkthrough uses.
const defaultDemoGoal = "Create an RM campaign agent that analyzes customer profile, " +
	"recommends suitable products, drafts a campaign message, and checks compliance before outreach."

// seed replays the whole demo: import the two Dify apps and the two manually registered
// capabilities, then register every candidate they produce.
func seed(ctx context.Context, client composer, opts options) error {
	fmt.Printf("seeding via %s\n", client.Describe())

	difyFiles := []string{"customer-insight-agent.dsl.yaml", "product-recommendation-workflow.dsl.yaml"}
	for _, file := range difyFiles {
		payload, err := os.ReadFile(filepath.Join(opts.dir, "dify", file))
		if err != nil {
			return err
		}
		result, err := client.ImportDify(ctx, "customer-dify", payload)
		if err != nil {
			return fmt.Errorf("import %s: %w", file, err)
		}
		reportImport(file, result)
	}

	manualFiles := []string{"compliance-check-skill.yaml", "campaign-message-template.yaml"}
	for _, file := range manualFiles {
		payload, err := os.ReadFile(filepath.Join(opts.dir, "capabilities", file))
		if err != nil {
			return err
		}
		result, err := client.ImportManual(ctx, "manual-registration", payload)
		if err != nil {
			return fmt.Errorf("import %s: %w", file, err)
		}
		reportImport(file, result)
	}

	return registerAll(ctx, client)
}

// registerAll promotes every pending candidate. Tools and datasets go first so that the
// agents and skills that bind them resolve their dependencies immediately.
func registerAll(ctx context.Context, client composer) error {
	order := []domain.CapabilityType{
		domain.CapabilityTypeTool,
		domain.CapabilityTypeKnowledgeData,
		domain.CapabilityTypePromptTemplate,
		domain.CapabilityTypeSkill,
		domain.CapabilityTypeWorkflow,
		domain.CapabilityTypeAgent,
		domain.CapabilityTypePolicy,
	}
	registered := 0
	for _, capType := range order {
		candidates, err := client.ListCandidates(ctx, "", string(capType))
		if err != nil {
			return err
		}
		for _, candidate := range candidates {
			if !candidate.Status.Pending() {
				continue
			}
			capability, err := client.Register(ctx, candidate.ID)
			if err != nil {
				// A duplicate is expected when the same asset arrives twice; report it
				// and keep going rather than abandoning the seed.
				fmt.Printf("  skipped %-32s %s\n", candidate.Name, err)
				continue
			}
			fmt.Printf("  registered %-16s %s\n", capability.Type, capability.Slug)
			registered++
		}
	}
	fmt.Printf("registered %d capabilities\n", registered)
	return nil
}

func runImport(ctx context.Context, client composer, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("import needs a kind: dify, manual, mcp or data")
	}
	readFile := func() ([]byte, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("import %s needs a file path", args[0])
		}
		return os.ReadFile(args[1])
	}

	var (
		result *engine.ImportResult
		err    error
		label  = args[0]
	)
	switch args[0] {
	case "dify":
		var payload []byte
		if payload, err = readFile(); err == nil {
			label = args[1]
			result, err = client.ImportDify(ctx, "customer-dify", payload)
		}
	case "manual":
		var payload []byte
		if payload, err = readFile(); err == nil {
			label = args[1]
			result, err = client.ImportManual(ctx, "manual-registration", payload)
		}
	case "mcp":
		result, err = client.ImportMockMCP(ctx)
	case "data":
		result, err = client.ImportMockData(ctx)
	default:
		return fmt.Errorf("unknown import kind %q", args[0])
	}
	if err != nil {
		return err
	}
	reportImport(label, result)
	return nil
}

func reportImport(label string, result *engine.ImportResult) {
	fmt.Printf("%s -> %d candidates\n", label, len(result.Candidates))
	for _, candidate := range result.Candidates {
		fmt.Printf("  %-16s %-32s %s\n", candidate.CandidateType, candidate.ExternalID, candidate.Status)
	}
	for _, warning := range result.Warnings {
		fmt.Printf("  warning: %s\n", warning)
	}
}

func listCandidates(ctx context.Context, client composer, opts options) error {
	candidates, err := client.ListCandidates(ctx, opts.status, opts.capType)
	if err != nil {
		return err
	}
	for _, candidate := range candidates {
		fmt.Printf("%-40s %-16s %-14s %s\n", candidate.ID, candidate.CandidateType, candidate.Status, candidate.Name)
	}
	fmt.Printf("%d candidates\n", len(candidates))
	return nil
}

func listCapabilities(ctx context.Context, client composer, opts options) error {
	capabilities, err := client.ListCapabilities(ctx, opts.capType)
	if err != nil {
		return err
	}
	for _, capability := range capabilities {
		fmt.Printf("%-16s %-32s %-10s %s\n",
			capability.Type, capability.Slug, capability.RiskLevel, strings.Join(capability.Intents, ","))
	}
	fmt.Printf("%d capabilities\n", len(capabilities))
	return nil
}

func draftCommand(ctx context.Context, client composer, goalText string, opts options) error {
	goal := goalFrom(goalText, opts)
	plan, err := client.Recommend(ctx, goal)
	if err != nil {
		return err
	}
	printPlan(plan)

	draft, err := client.GenerateDraft(ctx, engine.GenerateDraftRequest{
		Name:        opts.name,
		Goal:        goalText,
		HarnessType: goal.HarnessType,
		Plan:        *plan,
	})
	if err != nil {
		return err
	}
	fmt.Printf("\n--- %s (%s) ---\n%s", draft.Slug, draft.Status, draft.YAMLText)
	return nil
}

func printPlan(plan *domain.CompositionPlan) {
	sections := []struct {
		label string
		refs  []domain.CapabilityRef
	}{
		{"agents", plan.RecommendedAgents},
		{"workflows", plan.RecommendedWorkflows},
		{"skills", plan.RecommendedSkills},
		{"tools", plan.RecommendedTools},
		{"knowledge_data", plan.RecommendedKnowledgeData},
		{"prompts", plan.RecommendedPrompts},
		{"policies", plan.RecommendedPolicies},
	}
	for _, section := range sections {
		if len(section.refs) == 0 {
			continue
		}
		slugs := make([]string, 0, len(section.refs))
		for _, ref := range section.refs {
			slugs = append(slugs, ref.Slug)
		}
		fmt.Printf("%-16s %s\n", section.label, strings.Join(slugs, ", "))
	}
	fmt.Printf("%-16s %s\n", "workflow", strings.Join(plan.WorkflowSteps, " -> "))

	if len(plan.MissingCapabilities) > 0 {
		fmt.Println("missing:")
		for _, gap := range plan.MissingCapabilities {
			fmt.Printf("  %-24s needs a %s capability\n", gap.Step, gap.RequiredType)
		}
	}
	if len(plan.Warnings) > 0 {
		codes := make([]string, 0, len(plan.Warnings))
		for _, warning := range plan.Warnings {
			codes = append(codes, warning.Code)
		}
		sort.Strings(codes)
		fmt.Printf("%-16s %s\n", "warnings", strings.Join(codes, ", "))
	}
	fmt.Printf("%-16s risk=%s permissions=%s approvals=%s\n", "governance",
		plan.Governance.RiskLevel,
		strings.Join(plan.Governance.Permissions, ","),
		strings.Join(plan.Governance.ApprovalRequired, ","))
}
