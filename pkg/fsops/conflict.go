package fsops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type ConflictAction int

const (
	Skip ConflictAction = iota
	SkipAll
	Overwrite
	OverwriteAll
)

type Conflict struct {
	Source, Target string
	SourceInfo, TargetInfo os.FileInfo
}

// ConflictHandler runs on the operation worker; cancellation must unblock it.
type ConflictHandler func(context.Context, Conflict) (ConflictAction, error)

func (c *copier) resolve(src, dst string, source, target os.FileInfo) (ConflictAction, error) {
	if c.conflict == nil { return Overwrite, nil }
	if c.all == SkipAll { return Skip, nil }
	if c.all == OverwriteAll { return Overwrite, nil }
	a, err := c.conflict(c.ctx, Conflict{src, dst, source, target})
	if err != nil { return Skip, err }
	if err := c.ctx.Err(); err != nil { return Skip, err }
	switch a {
	case SkipAll: c.all = a; return Skip, nil
	case OverwriteAll: c.all = a; return Overwrite, nil
	case Skip, Overwrite: return a, nil
	default: return Skip, fmt.Errorf("invalid conflict decision: %d", a)
	}
}

// Stage type-changing replacements before moving the old target aside.
func (c *copier) replaceTree(src, dst string) error {
	stage, err := os.MkdirTemp(filepath.Dir(dst), ".dualcommander-*")
	if err != nil { return err }
	defer func() { if stage != "" { os.RemoveAll(stage) } }()
	move := c.move
	c.move = false
	err = c.copyTree(src, filepath.Join(stage, "new"))
	c.move = move
	if err != nil { return err }
	if err := c.ctx.Err(); err != nil { return err }
	old := filepath.Join(stage, "old")
	if err := os.Rename(dst, old); err != nil { return err }
	if err := os.Rename(filepath.Join(stage, "new"), dst); err != nil {
		if restore := os.Rename(old, dst); restore != nil {
			// Keep backup if restoring fails; never delete user's remaining copy.
			stage = ""
			return fmt.Errorf("replace: %v; restore: %v; original retained at %s", err, restore, old)
		}
		return err
	}
	if move { return os.RemoveAll(src) }
	return nil
}