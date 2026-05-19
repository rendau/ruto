package db

import domainModel "github.com/rendau/ruto/internal/domain/usr/model"

var (
	allowedSortFields = map[string]string{
		"id":       "id",
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

	if pars.Username != nil {
		conditions["username"] = *pars.Username
	}

	return conditions, conditionExps
}
