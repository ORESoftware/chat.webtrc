// source file path: ./src/common/mongo/transaction-3.go
package virl_mongo

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

func createTrn() {
	// Assuming `c.MongoDb` is your MongoDB client
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := (&mongo.Client{})
	session, err := client.StartSession()

	if err != nil {
		// handle error
		return // appropriate error response
	}
	defer session.EndSession(ctx)

	transactionFn := func(sc mongo.SessionContext) (interface{}, error) {
		// Transaction operations here, similar to your implementation
		// Ensure to use `sc` as the context for operations within the transactionFn

		// Commit is handled automatically by WithTransaction
		return nil, nil // or appropriate result
	}

	// Execute transactionFn
	if err := mongo.WithSession(ctx, session, func(sc mongo.SessionContext) error {
		// Commit Handling: The comment "// Commit is handled automatically by WithTransaction" is accurate.
		// When using WithTransaction, the MongoDB Go driver automatically attempts to commit the transaction
		// if the function completes without returning an error. If an error is returned, the transaction is aborted.
		_, err := session.WithTransaction(sc, transactionFn)
		return err
	}); err != nil {
		// handle error
		return // appropriate error response
	}

	// Success handling

}
