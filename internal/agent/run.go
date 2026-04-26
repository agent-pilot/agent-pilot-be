package agent

import (
	selector2 "agent-pilot-be/internal/agent/selector"
	agentcontext "agent-pilot-be/internal/context"
	"agent-pilot-be/internal/excutor"
	"agent-pilot-be/internal/llm"
	"agent-pilot-be/internal/prompt"
	"agent-pilot-be/internal/runtime"
	"agent-pilot-be/internal/session"
	"context"
	"fmt"
	"io"
	"strings"
)

type Agent struct {
	rt                *runtime.Runtime
	llm               *llm.LLM
	skillSelector     *selector2.SkillSelector
	referenceSelector *selector2.ReferenceSelector
	executor          *excutor.Executor
}

func NewAgent(rt *runtime.Runtime, l *llm.LLM, skillSelector *selector2.SkillSelector, referenceSelector *selector2.ReferenceSelector, executor *excutor.Executor) *Agent {
	return &Agent{
		rt:                rt,
		llm:               l,
		skillSelector:     skillSelector,
		referenceSelector: referenceSelector,
		executor:          executor,
	}
}

type Decision struct {
	Type                 string `json:"type"` // command|message
	Args                 string `json:"args,omitempty"`
	Content              string `json:"content,omitempty"`
	RequiresConfirmation bool   `json:"requires_confirmation,omitempty"`
	Why                  string `json:"why,omitempty"`
}

func isConfirmationOnly(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "确认", "好的", "好", "ok", "okay", "yes", "y", "继续", "继续执行":
		return true
	default:
		return false
	}
}

func lastNonConfirmationUserGoal(msgs []llm.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role != "user" {
			continue
		}
		t := strings.TrimSpace(m.Content)
		if t == "" {
			continue
		}
		if isConfirmationOnly(t) {
			continue
		}
		return t
	}
	return ""
}

// Advance runs one LLM planning step; if confirmation is required it transitions session to WaitingConfirm.
func (a *Agent) Advance(ctx context.Context, sess *session.Session, userQuery string, out io.Writer, errOut io.Writer) error {
	if sess == nil {
		return fmt.Errorf("nil session")
	}
	sess.Messages = append(sess.Messages, llm.Message{Role: "user", Content: userQuery})

	effectiveMessages := sess.Messages
	if isConfirmationOnly(userQuery) {
		if prev := lastNonConfirmationUserGoal(sess.Messages); prev != "" {
			merged := prev + "\n\n用户补充：确认（继续执行上一次请求，无需调整）。"
			effectiveMessages = append([]llm.Message(nil), sess.Messages...)
			effectiveMessages[len(effectiveMessages)-1] = llm.Message{Role: "user", Content: merged}
		}
	}

	// 1) select skills
	chosen, rawSel, err := a.skillSelector.Select(ctx, effectiveMessages, a.rt)
	if err != nil {
		fmt.Fprintln(errOut, "\nllm skill selection failed:", err)
		fmt.Fprintln(errOut, "raw:", rawSel)
		return nil
	}
	if len(chosen) == 0 {
		if isConfirmationOnly(userQuery) {
			fmt.Fprintln(out, "我收到了“确认”，但上一轮还没有进入可执行命令阶段。请补充你想确认的是：是否要创建PPT、页数、风格、标题等具体要求。")
			return nil
		}
		fmt.Fprintln(out, "我需要更具体的需求才能选择技能。请再描述一下你要做的操作。")
		return nil
	}
	fmt.Fprintln(out, "\nselected skills:", strings.Join(chosen, ", "))

	cb := agentcontext.NewBuilder(a.rt, a.referenceSelector)

	// 2) build skill context (lazy-load SKILL.md for chosen)
	skillCtx, err := cb.SkillContext(chosen)
	if err != nil {
		return err
	}

	// 3) reference selection + loading (include full message history)
	refCtx, _, rawRef, err := cb.References(ctx, chosen, skillCtx, effectiveMessages)
	if err != nil {
		fmt.Fprintln(errOut, "\nllm reference selection failed:", err)
		fmt.Fprintln(errOut, "raw:", rawRef)
		return err
	}

	// 4) plan one command
	msgs := prompt.BuildCommandPromptWithMessages(skillCtx, refCtx, effectiveMessages)
	var dec Decision
	rawCmd, err := a.llm.Chat(ctx, msgs, &dec)
	if err != nil {
		fmt.Fprintln(errOut, "\nllm command planning failed:", err)
		fmt.Fprintln(errOut, "raw:", rawCmd)
		return err
	}

	// 5) respond / interrupt / execute
	switch strings.TrimSpace(dec.Type) {
	case "message":
		fmt.Fprintln(out, dec.Content)
		sess.Messages = append(sess.Messages, llm.Message{Role: "assistant", Content: dec.Content})
		return nil
	case "command":
		if strings.TrimSpace(dec.Args) == "" {
			return fmt.Errorf("invalid llm decision raw: %s", rawCmd)
		}
		// Persist assistant plan into conversation history for next turn reasoning.
		sess.Messages = append(sess.Messages, llm.Message{Role: "assistant", Content: "proposed: lark-cli " + dec.Args})
		fmt.Fprintln(out, "\nproposed: lark-cli", dec.Args)
		if dec.RequiresConfirmation {
			sess.PendingCommand = dec.Args
			sess.State = session.StateWaitingConfirm
			return nil
		}
		res := a.executor.Run(ctx, dec.Args)
		if len(res.Stdout) > 0 {
			_, _ = out.Write(res.Stdout)
		}
		if len(res.Stderr) > 0 {
			_, _ = errOut.Write(res.Stderr)
		}
		if res.Err != nil {
			a.recordExecError(sess, dec.Args, res)
			fmt.Fprintln(out, "\n命令执行失败，我已记录错误信息。请继续输入下一步（例如：重试/换参数/查看帮助/改用其他命令）。")
			sess.State = session.StateWaitingInput
			return nil
		}
		sess.State = session.StateComplete
		return nil
	default:
		return fmt.Errorf("invalid llm decision raw: %s", rawCmd)
	}
}

// Confirm handles pending command confirmation for non-interactive orchestrators.
// If answer is "yes" it executes the pending command, otherwise cancels it.
func (a *Agent) Confirm(ctx context.Context, sess *session.Session, answer string, out io.Writer, errOut io.Writer) error {
	if sess == nil {
		return fmt.Errorf("nil session")
	}
	if sess.State != session.StateWaitingConfirm {
		return nil
	}
	ans := strings.TrimSpace(strings.ToLower(answer))
	if ans != "yes" {
		sess.PendingCommand = ""
		sess.State = session.StateWaitingInput
		if out != nil {
			fmt.Fprintln(out, "cancelled.")
		}
		return nil
	}
	res := a.executor.Run(ctx, sess.PendingCommand)
	if out != nil && len(res.Stdout) > 0 {
		_, _ = out.Write(res.Stdout)
	}
	if errOut != nil && len(res.Stderr) > 0 {
		_, _ = errOut.Write(res.Stderr)
	}
	if res.Err != nil {
		a.recordExecError(sess, sess.PendingCommand, res)
		if out != nil {
			fmt.Fprintln(out, "\n命令执行失败，我已记录错误信息。请继续输入下一步（例如：重试/换参数/查看帮助/改用其他命令）。")
		}
		sess.PendingCommand = ""
		sess.State = session.StateWaitingInput
		return nil
	}
	sess.PendingCommand = ""
	sess.State = session.StateComplete
	return nil
}

func (a *Agent) recordExecError(sess *session.Session, argstr string, res excutor.ExecResult) {
	if sess == nil {
		return
	}
	var b strings.Builder
	b.WriteString("command failed:\n")
	b.WriteString("lark-cli " + strings.TrimSpace(argstr) + "\n")
	if len(res.Stdout) > 0 {
		b.WriteString("\nstdout:\n")
		b.Write(res.Stdout)
	}
	if len(res.Stderr) > 0 {
		b.WriteString("\nstderr:\n")
		b.Write(res.Stderr)
	}
	if res.Err != nil {
		b.WriteString("\nerror:\n")
		b.WriteString(res.Err.Error())
	}
	sess.Messages = append(sess.Messages, llm.Message{Role: "assistant", Content: b.String()})
}
