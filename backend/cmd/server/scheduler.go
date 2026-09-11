package main

import (
	"context"
	"log"
	"time"
)

func (rt *Runtime) startScheduler(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if rt.archiveStore != nil {
				_ = rt.archiveStore.cleanup()
			}
			rt.runDueParses(ctx)
		}
	}
}

func (rt *Runtime) runDueParses(ctx context.Context) {
	_, db, ok := rt.deps()
	if !ok {
		return
	}
	var sources []DocumentSource
	if err := db.Where("enabled = ? AND (next_run_at IS NULL OR next_run_at <= ?)", true, time.Now()).
		Order("next_run_at asc").
		Limit(10).
		Find(&sources).Error; err != nil {
		log.Printf("scheduler list sources: %v", err)
		return
	}
	for i := range sources {
		if ctx.Err() != nil {
			return
		}
		source := sources[i]
		started := time.Now()
		result, parseErr := rt.parseTencentDoc(ctx, source.URL)
		if _, err := saveParseResult(db, &source, result, started, parseErr); err != nil {
			log.Printf("scheduler save source %d: %v", source.ID, err)
		}
	}
}
