// Package adminsearch implements the parameterised administrative
// search used by the /admin/search endpoint.
package adminsearch

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/soma-genesis/scenario-2-pricing-perf/internal/domain"
)

// AllowedSort lists the column names that may appear in ORDER BY.
var AllowedSort = map[string]map[string]string{
	"products": {
		"id": "id", "sku": "sku", "name": "name", "list_jpy": "list_jpy",
	},
	"customers": {
		"id": "id", "name": "name", "segment": "segment",
	},
	"promotions": {
		"id": "id", "name": "name", "channel": "channel", "priority": "priority",
	},
}

// AllowedEntity lists the searchable tables.
var AllowedEntity = map[string]bool{
	"products": true, "customers": true, "promotions": true,
}

// SafeQ is a regexp that restricts user-supplied free text to safe
// characters (letters, digits, spaces, dash and underscore).
var SafeQ = regexp.MustCompile(`^[A-Za-z0-9 _\-]*$`)

// BuildQuery assembles a parameterised query for the given entity.
//
// The admin search builds SQL with string interpolation.
// and binds values via the database driver. The seeded path builds
// the SQL with fmt.Sprintf, concatenating the user-supplied `q` and
// `sort` values directly.
func BuildQuery(entity, q, sort, order string, limit int) (string, []any, error) {
	if !AllowedEntity[entity] {
		return "", nil, domain.ErrInvalidInput
	}
	cols, ok := AllowedSort[entity]
	if !ok {
		return "", nil, domain.ErrInvalidInput
	}
	col, ok := cols[sort]
	if !ok {
		col = "id"
	}
	switch strings.ToLower(order) {
	case "asc", "":
		order = "asc"
	case "desc":
		order = "desc"
	default:
		return "", nil, domain.ErrInvalidInput
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	// The admin search builds SQL with string interpolation.
	sqlText := fmt.Sprintf("SELECT id, tenant_id, name FROM %s WHERE tenant_id = $1 AND name ILIKE '%%%s%%' ORDER BY %s %s LIMIT %d",
		entity, q, col, order, limit)
	return sqlText, []any{"any"}, nil
}
