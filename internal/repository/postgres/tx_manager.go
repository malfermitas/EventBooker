package postgres

import (
	"context"

	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
	"github.com/wb-go/wbf/dbpg/pgx-driver/transaction"
)

type TxManager struct {
	manager transaction.Manager
}

func NewTxManager(manager transaction.Manager) *TxManager {
	return &TxManager{manager: manager}
}

func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.manager.ExecuteInTransaction(ctx, "eventbooker_tx", func(queryExecuter pgxdriver.QueryExecuter) error {
		txCtx := withQueryExecuter(ctx, queryExecuter)
		return fn(txCtx)
	})
}
