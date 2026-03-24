package postgres

import (
	"context"

	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
)

type queryExecuterContextKey struct{}

func withQueryExecuter(ctx context.Context, queryExecuter pgxdriver.QueryExecuter) context.Context {
	return context.WithValue(ctx, queryExecuterContextKey{}, queryExecuter)
}

func getQueryExecuter(ctx context.Context, db pgxdriver.QueryExecuter) pgxdriver.QueryExecuter {
	if queryExecuter, ok := ctx.Value(queryExecuterContextKey{}).(pgxdriver.QueryExecuter); ok && queryExecuter != nil {
		return queryExecuter
	}

	return db
}
