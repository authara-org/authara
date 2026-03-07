package testutil_test

import (
	"context"
	"testing"

	"github.com/alexlup06-authgate/authgate/internal/testutil"
)

func TestWithRollbackTx_StartsTransaction(t *testing.T) {
	tdb := testutil.OpenTestDB(t)

	testutil.WithRollbackTx(t, tdb, func(ctx context.Context) {
		if ctx == nil {
			t.Fatal("expected non-nil context")
		}
	})
}
