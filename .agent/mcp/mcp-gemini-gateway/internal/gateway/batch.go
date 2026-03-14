package gateway

import (
	"context"
	"fmt"
	"sync"

	"github.com/midweste/dotai/mcp-gemini-gateway/internal/config"
	"github.com/midweste/dotai/mcp-gemini-gateway/internal/domain"
)

// FindBucketForModel returns the bucket containing the given alias, or nil.
func FindBucketForModel(cfg *config.Config, alias string) []string {
	for _, bucket := range cfg.ModelBuckets {
		for _, m := range bucket {
			if m == alias {
				return bucket
			}
		}
	}
	return nil
}

// Assignment pairs a job index with its assigned model alias.
type Assignment struct {
	Index int
	Alias string
}

// AssignModelsForBatch assigns concrete model aliases to batch jobs for max parallelism.
func (g *Gateway) AssignModelsForBatch(ctx context.Context, jobs []DispatchRequest) []Assignment {
	runningModels, _ := g.store.RunningModels(ctx)
	runningSet := make(map[string]bool)
	for _, m := range runningModels {
		runningSet[g.registry.AliasFor(m)] = true
	}
	assigned := make(map[string]bool)
	for k := range runningSet {
		assigned[k] = true
	}

	result := make([]Assignment, 0, len(jobs))

	for i, job := range jobs {
		requested := job.Model
		if requested == "" {
			requested = "fast"
		}
		bucket := FindBucketForModel(g.cfg, requested)

		if bucket != nil && !assigned[requested] {
			assigned[requested] = true
			result = append(result, Assignment{i, requested})
			continue
		}

		if bucket != nil {
			reqIdx := indexOf(bucket, requested)
			found := false

			// Try smarter first (higher index), then lesser
			for _, m := range bucket {
				if indexOf(bucket, m) > reqIdx && !assigned[m] {
					assigned[m] = true
					result = append(result, Assignment{i, m})
					found = true
					break
				}
			}
			if !found {
				for _, m := range bucket {
					if indexOf(bucket, m) < reqIdx && !assigned[m] {
						assigned[m] = true
						result = append(result, Assignment{i, m})
						found = true
						break
					}
				}
			}
			if !found {
				result = append(result, Assignment{i, requested})
			}
		} else {
			result = append(result, Assignment{i, requested})
		}
	}

	return result
}

// RunBatch dispatches multiple jobs with goroutines for parallel model slots.
func (g *Gateway) RunBatch(ctx context.Context, jobs []DispatchRequest) ([]domain.BatchResult, error) {
	assignments := g.AssignModelsForBatch(ctx, jobs)

	// Group by assigned model
	type indexedJob struct {
		index int
		job   DispatchRequest
	}
	modelGroups := make(map[string][]indexedJob)
	for _, a := range assignments {
		jobs[a.Index].Model = a.Alias
		modelGroups[a.Alias] = append(modelGroups[a.Alias], indexedJob{a.Index, jobs[a.Index]})
	}

	batchID := fmt.Sprintf("%08x", uint32(len(jobs)))

	// Results collected concurrently
	results := make([]domain.BatchResult, len(jobs))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, group := range modelGroups {
		wg.Add(1)
		go func(group []indexedJob) {
			defer wg.Done()
			// Jobs within same model run serially
			for _, ij := range group {
				ij.job.BatchID = batchID
				result, err := g.Dispatch(ctx, ij.job)

				var br domain.BatchResult
				br.Label = ij.job.Label
				br.Model = ij.job.Model
				if err != nil {
					br.Status = fmt.Sprintf("error: %v", err)
					br.ExitCode = 1
				} else {
					br.ExitCode = result.ExitCode
					if result.ExitCode == 0 {
						br.Status = "ok"
					} else {
						br.Status = fmt.Sprintf("exit=%d", result.ExitCode)
					}
				}

				mu.Lock()
				results[ij.index] = br
				mu.Unlock()
			}
		}(group)
	}

	wg.Wait()
	return results, nil
}

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}
