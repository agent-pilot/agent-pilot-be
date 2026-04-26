package agent

import (
	planner2 "agent-pilot-be/internal/agent/planner"
	"agent-pilot-be/internal/llm"
	"agent-pilot-be/internal/session"
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// Workflow turns one user goal into multiple step sessions.
type Workflow struct {
	planner *planner2.Planner
	agent   *Agent

	plan    planner2.Plan
	stepIdx int

	sessions []*session.Session
	started  []bool
}

func NewWorkflow(p *planner2.Planner, a *Agent) *Workflow {
	return &Workflow{
		planner: p,
		agent:   a,
	}
}

func (wf *Workflow) Init(ctx context.Context, userGoal string) error {
	if wf == nil || wf.planner == nil || wf.agent == nil {
		return fmt.Errorf("nil workflow deps")
	}
	msgs := []llm.Message{{Role: "user", Content: strings.TrimSpace(userGoal)}}
	plan, raw, err := wf.planner.CreatePlan(ctx, msgs)
	if err != nil {
		return fmt.Errorf("create plan failed: %w; raw=%s", err, raw)
	}
	wf.plan = plan
	wf.stepIdx = 0
	wf.sessions = make([]*session.Session, len(plan.Steps))
	wf.started = make([]bool, len(plan.Steps))
	for i := range wf.sessions {
		wf.sessions[i] = &session.Session{State: session.StateWaitingInput}
	}
	return nil
}

func (wf *Workflow) RunInteractive(ctx context.Context, in io.Reader, out io.Writer, errOut io.Writer) error {
	if wf == nil {
		return fmt.Errorf("nil workflow")
	}
	if in == nil || out == nil || errOut == nil {
		return fmt.Errorf("nil io")
	}
	sc := bufio.NewScanner(in)

	for wf.stepIdx < len(wf.plan.Steps) {
		step := wf.plan.Steps[wf.stepIdx]
		sess := wf.sessions[wf.stepIdx]

		// Kick off step once with its goal as the first "user input".
		if !wf.started[wf.stepIdx] {
			wf.started[wf.stepIdx] = true
			fmt.Fprintf(out, "\n== Step %d/%d: %s ==\n", wf.stepIdx+1, len(wf.plan.Steps), step.Name)
			fmt.Fprintln(out, step.Goal)
			if err := wf.agent.Advance(ctx, sess, step.Goal, out, errOut); err != nil {
				return err
			}
			// If step completed immediately, go next.
			if sess.State == session.StateComplete {
				wf.stepIdx++
				continue
			}
		}

		switch sess.State {
		case session.StateWaitingConfirm:
			fmt.Fprint(out, "type 'yes' to run: ")
			if !sc.Scan() {
				return nil
			}
			ans := strings.TrimSpace(sc.Text())
			if err := wf.agent.Confirm(ctx, sess, ans, out, errOut); err != nil {
				return err
			}
			if sess.State == session.StateComplete {
				wf.stepIdx++
			}

		case session.StateWaitingInput:
			fmt.Fprint(out, "step> ")
			if !sc.Scan() {
				return nil
			}
			q := strings.TrimSpace(sc.Text())
			if q == "" {
				continue
			}
			if q == "exit" || q == "quit" || q == ":q" || q == ":quit" {
				return nil
			}
			if err := wf.agent.Advance(ctx, sess, q, out, errOut); err != nil {
				return err
			}
			if sess.State == session.StateComplete {
				wf.stepIdx++
			}

		case session.StateComplete:
			wf.stepIdx++

		default:
			sess.State = session.StateWaitingInput
		}
	}

	fmt.Fprintln(out, "\nworkflow complete.")
	return nil
}
