package engine

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"detection_engine/internal/catalog"
	"detection_engine/internal/config"
	"detection_engine/internal/index"
	"detection_engine/internal/linguist"
	"detection_engine/internal/manifests"
	"detection_engine/internal/rules"
	"detection_engine/internal/score"
	"detection_engine/internal/stack"
)

type Engine struct {
	cfg config.Config
	sem *semaphore.Weighted
}

func New(cfg config.Config) *Engine {
	return &Engine{
		cfg: cfg,
		sem: semaphore.NewWeighted(int64(cfg.MaxConcurrent)),
	}
}

func (e *Engine) DetectDir(ctx context.Context, requestID, dir string, source stack.Source) (*stack.Result, error) {
	if err := e.sem.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer e.sem.Release(1)

	started := time.Now()
	ctx, cancel := context.WithTimeout(ctx, e.cfg.DetectTimeout)
	defer cancel()

	idx, err := index.Build(ctx, dir, e.cfg)
	if err != nil {
		return nil, fmt.Errorf("index repository: %w", err)
	}

	g, gctx := errgroup.WithContext(ctx)
	var langs []stack.Language
	var set *manifests.Set
	var hits []catalog.Hit

	g.Go(func() error {
		var err error
		langs, err = linguist.Analyze(gctx, idx, e.cfg)
		return err
	})
	g.Go(func() error {
		var err error
		set, err = manifests.Parse(gctx, idx, e.cfg)
		return err
	})
	g.Go(func() error {
		hits = catalog.Scan(idx)
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	candidates := rules.Match(idx, set)
	apps := score.BuildApps(idx, set, langs, candidates)

	if source.Type == "" {
		source.Type = stack.SourceDir
	}
	if langs == nil {
		langs = []stack.Language{}
	}
	infra := catalog.Infra(hits)
	if infra == nil {
		infra = []stack.Tech{}
	}
	warnings := score.Warnings(apps)
	if warnings == nil {
		warnings = []stack.Warning{}
	}

	return &stack.Result{
		RequestID:  requestID,
		DurationMS: time.Since(started).Milliseconds(),
		Source:     source,
		Languages:  langs,
		Apps:       apps,
		Infra:      infra,
		Warnings:   warnings,
	}, nil
}
