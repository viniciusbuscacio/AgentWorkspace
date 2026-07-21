package main

import (
	"context"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type PipControl struct {
	ctx context.Context
}

func (p *PipControl) startup(ctx context.Context) {
	p.ctx = ctx
}

func (p *PipControl) Close() {
	if p.ctx != nil {
		wailsruntime.Quit(p.ctx)
	}
}
