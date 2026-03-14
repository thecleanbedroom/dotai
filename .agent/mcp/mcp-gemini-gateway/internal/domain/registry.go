package domain

import "fmt"

// ModelRegistry provides DRY model alias ↔ full name lookups.
// Eliminates the 6× repeated reverse-lookup from the Python codebase.
type ModelRegistry struct {
	aliasToModel map[string]string
	modelToAlias map[string]string
}

// NewModelRegistry creates a registry from an alias→model map.
func NewModelRegistry(models map[string]string) *ModelRegistry {
	r := &ModelRegistry{
		aliasToModel: make(map[string]string, len(models)),
		modelToAlias: make(map[string]string, len(models)),
	}
	for alias, model := range models {
		r.aliasToModel[alias] = model
		r.modelToAlias[model] = alias
	}
	return r
}

// Resolve converts a short alias to the full Gemini model string.
// Returns an error if the alias is unknown.
func (r *ModelRegistry) Resolve(alias string) (string, error) {
	model, ok := r.aliasToModel[alias]
	if !ok {
		return "", fmt.Errorf("unknown model alias %q, valid: %v", alias, r.Aliases())
	}
	return model, nil
}

// MustResolve converts a short alias to the full model string, panicking on error.
// Use only during initialization where failure is unrecoverable.
func (r *ModelRegistry) MustResolve(alias string) string {
	model, err := r.Resolve(alias)
	if err != nil {
		panic(err)
	}
	return model
}

// AliasFor returns the short alias for a full model name, or the model name itself if unknown.
func (r *ModelRegistry) AliasFor(fullName string) string {
	if alias, ok := r.modelToAlias[fullName]; ok {
		return alias
	}
	return fullName
}

// ForEach iterates over all alias→model pairs.
func (r *ModelRegistry) ForEach(fn func(alias, model string)) {
	for alias, model := range r.aliasToModel {
		fn(alias, model)
	}
}

// Aliases returns all known aliases.
func (r *ModelRegistry) Aliases() []string {
	aliases := make([]string, 0, len(r.aliasToModel))
	for alias := range r.aliasToModel {
		aliases = append(aliases, alias)
	}
	return aliases
}

// HasAlias returns true if the alias exists.
func (r *ModelRegistry) HasAlias(alias string) bool {
	_, ok := r.aliasToModel[alias]
	return ok
}
