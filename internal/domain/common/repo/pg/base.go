package common

import (
	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rendau/mobone/v2"
)

type Base struct {
	Con *pgxpool.Pool
	// Txm makes queries join the transaction stored in ctx (if any): every
	// TransactionManager reads the same mobone ctx key, so a usecase-level
	// TxFn spans all repos.
	Txm *mobone.TransactionManager
	QB  squirrel.StatementBuilderType
}

func NewBase(con *pgxpool.Pool) *Base {
	return &Base{
		Con: con,
		Txm: mobone.NewTransactionManager(con),
		QB:  squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}
