package app

import (
	"fmt"
	"time"

	"github.com/DaleXiao/slackogo/internal/api"
	"github.com/DaleXiao/slackogo/internal/auth"
	"github.com/DaleXiao/slackogo/internal/output"
)

type Context struct {
	Printer   *output.Printer
	Workspace string
	Timeout   time.Duration
	Verbose   bool
	Debug     bool
	Quiet     bool
}

func (ctx *Context) NewClient() (*api.Client, error) {
	creds, err := auth.FindCredentials(ctx.Workspace)
	if err != nil {
		return nil, fmt.Errorf("auth error: %w", err)
	}
	return api.NewClient(creds, ctx.Timeout), nil
}
