package db

import (
	"strings"

	domainModel "github.com/rendau/ruto/internal/domain/usr/model"
)

var (
	allowedSortFields = map[string]string{
		"id":       "id",
		"active":   "active",
		"is_admin": "is_admin",
		"name":     "name",
		"username": "username",
	}
)

func (r *Repo) getConditions(pars *domainModel.ListReq) (map[string]any, map[string][]any) {
	conditions := make(map[string]any)
	conditionExps := make(map[string][]any)

	if pars == nil {
		return conditions, conditionExps
	}

	if pars.Search != nil {
		query := strings.TrimSpace(*pars.Search)
		if query != "" {
			pattern := "%" + query + "%"
			conditionExps["(username ILIKE ? OR name ILIKE ?)"] = []any{pattern, pattern}
		}
	}

	return conditions, conditionExps
}
