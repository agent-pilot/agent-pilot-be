package main

import (
	planner2 "agent-pilot-be/internal/agent/planner"
	"agent-pilot-be/internal/agent/selector"
	"agent-pilot-be/internal/excutor"
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"agent-pilot-be/configs"
	agent2 "agent-pilot-be/internal/agent"
	"agent-pilot-be/internal/llm"
	"agent-pilot-be/internal/runtime"
)

func main() {
	ctx := context.Background()
	rt := runtime.NewRuntime(ctx)

	conf, err := configs.Load("configs/config.yaml")
	if err != nil {
		fmt.Fprintln(os.Stderr, "load config failed:", err)
		os.Exit(1)
	}
	client, err := llm.NewClient(conf)
	if err != nil {
		fmt.Fprintln(os.Stderr, "init llm client failed:", err)
		os.Exit(1)
	}

	skillSelector := selector.NewSkillSelector(client)
	referenceSelector := selector.NewReferenceSelector(client)
	executor := excutor.NewExecutor()
	a := agent2.NewAgent(rt, client, skillSelector, referenceSelector, executor)

	fmt.Println("agent runtime ok:", rt.DebugSummary())
	fmt.Print("\ngoal> ")
	sc := bufio.NewScanner(os.Stdin)
	if !sc.Scan() {
		return
	}
	goal := strings.TrimSpace(sc.Text())
	if goal == "" {
		return
	}

	p := planner2.NewPlanner(client)
	wf := agent2.NewWorkflow(p, a)
	if err := wf.Init(ctx, goal); err != nil {
		fmt.Fprintln(os.Stderr, "\nplanner init failed:", err)
		os.Exit(1)
	}
	if err := wf.RunInteractive(ctx, os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "\nworkflow failed:", err)
		os.Exit(1)
	}
}
