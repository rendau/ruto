package db

import "github.com/rendau/ruto/internal/domain/endpoint/model"

var (
	allowedSortFields = map[string]string{
		"active": "active",
		"id":     "id",
		"app_id": "app_id",
		"method": "method",
		"path":   "path",
	}
)

func (r *Repo) getConditions(pars *model.ListReq) (map[string]any, map[string][]any) {
	conditions := make(map[string]any)
	conditionExps := make(map[string][]any)

	if pars == nil {
		return conditions, conditionExps
	}

	if pars.AppId != nil {
		conditions["app_id"] = *pars.AppId
	}
	if pars.Active != nil {
		conditions["active"] = *pars.Active
	}

	return conditions, conditionExps
}
