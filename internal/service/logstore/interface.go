package logstore

import (
	"context"

	logstoreModel "github.com/rendau/ruto/internal/service/logstore/model"
)

type LogStore interface {
	List(ctx context.Context, pars *logstoreModel.ListReq) ([]*logstoreModel.Entry, error)
}
