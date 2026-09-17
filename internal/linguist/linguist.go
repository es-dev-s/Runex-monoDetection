package linguist

import (
	"context"
	"sort"

	enry "github.com/go-enry/go-enry/v2"
	"golang.org/x/sync/errgroup"

	"detection_engine/internal/config"
	"detection_engine/internal/index"
	"detection_engine/internal/stack"
)

type hit struct {
	lang  string
	bytes int64
}

func Analyze(ctx context.Context, idx *index.Index, cfg config.Config) ([]stack.Language, error) {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(cfg.LinguistWorkers)
	hits := make(chan hit, cfg.LinguistWorkers*2)

	g.Go(func() error {
		defer close(hits)
		inner, innerCtx := errgroup.WithContext(ctx)
		inner.SetLimit(cfg.LinguistWorkers)
		for i := range idx.Files {
			file := idx.Files[i]
			if file.Size == 0 {
				continue
			}
			inner.Go(func() error {
				if err := innerCtx.Err(); err != nil {
					return err
				}
				if enry.IsVendor(file.Rel) || enry.IsDocumentation(file.Rel) || enry.IsConfiguration(file.Rel) {
					return nil
				}
				limit := cfg.MaxFileReadBytes
				if file.Size < limit {
					limit = file.Size
				}
				if limit > 32<<10 {
					limit = 32 << 10
				}
				data, err := index.ReadAbs(file.Abs, limit)
				if err != nil {
					return nil
				}
				if enry.IsBinary(data) || enry.IsGenerated(file.Rel, data) {
					return nil
				}
				lang := enry.GetLanguage(file.Name, data)
				if lang == "" || lang == "Text" || lang == "Markdown" {
					return nil
				}
				select {
				case hits <- hit{lang: lang, bytes: file.Size}:
					return nil
				case <-innerCtx.Done():
					return innerCtx.Err()
				}
			})
		}
		return inner.Wait()
	})

	totals := make(map[string]int64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for h := range hits {
			totals[h.lang] += h.bytes
		}
	}()

	if err := g.Wait(); err != nil {
		return nil, err
	}
	<-done

	var sum int64
	for _, n := range totals {
		sum += n
	}
	out := make([]stack.Language, 0, len(totals))
	for name, n := range totals {
		pct := 0.0
		if sum > 0 {
			pct = float64(n) * 100 / float64(sum)
		}
		out = append(out, stack.Language{Name: name, Bytes: n, Percentage: pct})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bytes == out[j].Bytes {
			return out[i].Name < out[j].Name
		}
		return out[i].Bytes > out[j].Bytes
	})
	if len(out) > 12 {
		out = out[:12]
	}
	return out, nil
}

func Primary(langs []stack.Language) string {
	if len(langs) == 0 {
		return ""
	}
	return langs[0].Name
}
