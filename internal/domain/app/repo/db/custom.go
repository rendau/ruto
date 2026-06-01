package db

import "github.com/rendau/ruto/internal/domain/app/model"

var (
	allowedSortFields = map[string]string{
		"id":   "id",
		"name": "data ->> 'name'",
	}
)

func (r *Repo) getConditions(pars *model.ListReq) (map[string]any, map[string][]any) {
	conditions := make(map[string]any)
	conditionExps := make(map[string][]any)

	if pars == nil {
		return conditions, conditionExps
	}

	if pars.Active != nil {
		conditions["active"] = *pars.Active
	}
	if pars.NameEqCI != nil {
		conditionExps["LOWER(data ->> 'name') = LOWER(?)"] = []any{*pars.NameEqCI}
	}
	if pars.ExcludeID != nil {
		conditionExps["id != ?"] = []any{*pars.ExcludeID}
	}

	return conditions, conditionExps
}
