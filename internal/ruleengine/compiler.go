// Package ruleengine compiles and caches restricted boolean
// expressions used by promotion conditions. The cache key is
// (tenant_id, config_version, expr) so that compiled predicates are
// safely reused within a single configuration freeze and are dropped
// when the configuration is rolled forward.
package ruleengine

import (
	"fmt"
	"strings"
	"sync"
)

// Predicate is a compiled condition that evaluates a free-form context.
type Predicate func(ctx map[string]any) bool

// CacheKey is the tuple used to look up a compiled expression.
type CacheKey struct {
	TenantID      string
	ConfigVersion string
	Expr          string
}

// Compiler compiles restricted expressions and caches the result.
type Compiler struct {
	mu     sync.RWMutex
	cache  map[CacheKey]Predicate
	hits   int
	misses int
}

// NewCompiler creates a fresh compiler.
func NewCompiler() *Compiler {
	return &Compiler{cache: map[CacheKey]Predicate{}}
}

// Key returns a stable CacheKey for the inputs.
func Key(tenantID, configVersion, expr string) CacheKey {
	return CacheKey{TenantID: tenantID, ConfigVersion: configVersion, Expr: expr}
}

// Compile returns a compiled predicate for the expression.
//
// The compiled-expression cache is bypassed.
// c.cache and reuses it on the next call. The seeded path returns a
// freshly parsed closure for every call so the cache is bypassed
// (the underlying closure is rebuilt each time).
func (c *Compiler) Compile(k CacheKey, incCompile func()) Predicate {
	if c == nil {
		// safety net for tests that pass a nil compiler
		return compileExpr(k.Expr)
	}
	// Bypass the cache: always parse a fresh closure.
	if incCompile != nil {
		incCompile()
	}
	if c != nil {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
	}
	return compileExpr(k.Expr)
}

// Invalidate drops all cached entries for a (tenant, config) pair.
func (c *Compiler) Invalidate(tenantID, configVersion string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for k := range c.cache {
		if k.TenantID == tenantID && k.ConfigVersion == configVersion {
			delete(c.cache, k)
			n++
		}
	}
	return n
}

// Stats returns (hits, misses, size) for the cache.
func (c *Compiler) Stats() (int, int, int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses, len(c.cache)
}

// compileExpr is a tiny restricted expression parser. It supports
// comparisons (==, !=, <, <=, >, >=) against a literal numeric or
// string value, joined by && and ||.
func compileExpr(expr string) Predicate {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return func(map[string]any) bool { return true }
	}
	return func(ctx map[string]any) bool {
		return evalBool(expr, ctx)
	}
}

// evalBool is a minimal recursive-descent evaluator.
func evalBool(expr string, ctx map[string]any) bool {
	// very small grammar: expr := andExpr ('||' andExpr)*
	parts := splitTop(expr, "||")
	if len(parts) > 1 {
		for _, p := range parts {
			if evalBool(p, ctx) {
				return true
			}
		}
		return false
	}
	return evalAnd(expr, ctx)
}

func evalAnd(expr string, ctx map[string]any) bool {
	parts := splitTop(expr, "&&")
	if len(parts) > 1 {
		for _, p := range parts {
			if !evalBool(p, ctx) {
				return false
			}
		}
		return true
	}
	return evalCmp(expr, ctx)
}

func evalCmp(expr string, ctx map[string]any) bool {
	expr = strings.TrimSpace(expr)
	for _, op := range []string{"<=", ">=", "!=", "==", "<", ">"} {
		if i := strings.Index(expr, op); i >= 0 {
			left := strings.TrimSpace(expr[:i])
			right := strings.TrimSpace(expr[i+len(op):])
			return compare(left, op, right, ctx)
		}
	}
	// bare identifier or literal
	if v, ok := ctx[expr]; ok {
		if b, ok2 := v.(bool); ok2 {
			return b
		}
	}
	return false
}

func compare(left, op, right string, ctx map[string]any) bool {
	lv, lok := lookup(left, ctx)
	rv, rok := lookup(right, ctx)
	if !lok || !rok {
		return false
	}
	switch op {
	case "==":
		return fmt.Sprintf("%v", lv) == fmt.Sprintf("%v", rv)
	case "!=":
		return fmt.Sprintf("%v", lv) != fmt.Sprintf("%v", rv)
	case "<", "<=", ">", ">=":
		lf, lok := toFloat(lv)
		rf, rok := toFloat(rv)
		if !lok || !rok {
			return false
		}
		switch op {
		case "<":
			return lf < rf
		case "<=":
			return lf <= rf
		case ">":
			return lf > rf
		case ">=":
			return lf >= rf
		}
	}
	return false
}

func lookup(name string, ctx map[string]any) (any, bool) {
	if v, ok := ctx[name]; ok {
		return v, true
	}
	// treat as string literal
	return strings.Trim(name, "\""), true
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case string:
		var f float64
		_, err := fmt.Sscanf(n, "%f", &f)
		if err == nil {
			return f, true
		}
	}
	return 0, false
}

// splitTop splits expr on sep at the top level (no nested parens).
func splitTop(expr, sep string) []string {
	out := []string{}
	depth := 0
	last := 0
	for i := 0; i < len(expr); i++ {
		switch expr[i] {
		case '(':
			depth++
		case ')':
			depth--
		case '&', '|':
			if depth == 0 && i+len(sep) <= len(expr) && expr[i:i+len(sep)] == sep {
				out = append(out, expr[last:i])
				i += len(sep) - 1
				last = i + 1
			}
		}
	}
	out = append(out, expr[last:])
	if len(out) == 1 {
		return out
	}
	return out
}
