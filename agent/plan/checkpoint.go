package plan

import (
	"context"
	"fmt"
	"sync"
	"time"

	atype "github.com/agent-pilot/agent-pilot-be/agent/type"
)

type Checkpoint struct {
	ID        string    `json:"id"`
	Plan      *atype.Plan `json:"plan"`
	StepID    string    `json:"step_id,omitempty"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

type Checkpointer interface {
	Save(ctx context.Context, plan *atype.Plan, reason string) (*Checkpoint, error)
	Load(ctx context.Context, id string) (*Checkpoint, error)
	Latest(ctx context.Context, sessionID string) (*Checkpoint, error)
}

type MemoryCheckpointer struct {
	mu              sync.RWMutex
	items           map[string]*Checkpoint
	latestBySession map[string]string
	now             func() time.Time
}

func NewMemoryCheckpointer() *MemoryCheckpointer {
	return &MemoryCheckpointer{
		items:           make(map[string]*Checkpoint),
		latestBySession: make(map[string]string),
		now:             time.Now,
	}
}

func (c *MemoryCheckpointer) Save(ctx context.Context, plan *atype.Plan, reason string) (*Checkpoint, error) {
	return c.SaveStep(ctx, plan, "", reason)
}

func (c *MemoryCheckpointer) SaveStep(ctx context.Context, plan *atype.Plan, stepID, reason string) (*Checkpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, fmt.Errorf("plan is nil")
	}
	if plan.ID == "" {
		return nil, fmt.Errorf("plan id is required")
	}

	cp := &Checkpoint{
		ID:        NewID("checkpoint"),
		Plan:      clonePlan(plan),
		StepID:    stepID,
		Reason:    reason,
		CreatedAt: c.now(),
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[cp.ID] = cp
	if plan.SessionID != "" {
		c.latestBySession[plan.SessionID] = cp.ID
	}

	return cloneCheckpoint(cp), nil
}

func (c *MemoryCheckpointer) Load(ctx context.Context, id string) (*Checkpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	cp, ok := c.items[id]
	if !ok {
		return nil, fmt.Errorf("checkpoint not found: %s", id)
	}
	return cloneCheckpoint(cp), nil
}

func (c *MemoryCheckpointer) Latest(ctx context.Context, sessionID string) (*Checkpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	id, ok := c.latestBySession[sessionID]
	if !ok {
		return nil, fmt.Errorf("checkpoint not found for session: %s", sessionID)
	}
	cp, ok := c.items[id]
	if !ok {
		return nil, fmt.Errorf("checkpoint not found: %s", id)
	}
	return cloneCheckpoint(cp), nil
}

func cloneCheckpoint(cp *Checkpoint) *Checkpoint {
	if cp == nil {
		return nil
	}
	return &Checkpoint{
		ID:        cp.ID,
		Plan:      clonePlan(cp.Plan),
		StepID:    cp.StepID,
		Reason:    cp.Reason,
		CreatedAt: cp.CreatedAt,
	}
}

func clonePlan(in *atype.Plan) *atype.Plan {
	if in == nil {
		return nil
	}
	out := *in
	out.Steps = append([]atype.Step(nil), in.Steps...)
	return &out
}
