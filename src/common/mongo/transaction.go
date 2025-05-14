// source file path: ./src/common/mongo/transaction.go
package virl_mongo

import (
	"context"
	vbl "github.com/oresoftware/chat.webtrc/src/common/vibelog"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"time"
)

type ICommitError interface {
	Error() string
}

type CommitError struct {
	wrappedError error
}

func (c *CommitError) Error() string {
	return c.wrappedError.Error()
}

func DoTrx(to int, client *mongo.Client, criticalSection func(sessionContext *mongo.SessionContext) error) error {
	// MongoDB connection

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(to)*time.Second)
	defer func() {
		cancel()
	}()

	// Create a session to start a transaction
	session, err := client.StartSession()

	if err != nil {
		vbl.Stdout.Warn(vbl.Id("vid/56d76d03c806"), err)
		return err
	}

	defer func() {
		session.EndSession(context.Background())
	}()

	// Start a transaction and execute the critical section
	if err := mongo.WithSession(ctx, session, func(sc mongo.SessionContext) error {
		// Set write concern and read preference

		// Set up transaction options
		txnOpts := options.Transaction().
			SetReadPreference(readpref.Primary()).   // Set Read Preference
			SetWriteConcern(writeconcern.Majority()) // Set Write Concern using the updated method

		// try session.WithTransaction(sc,
		if err := sc.StartTransaction(txnOpts); err != nil {
			vbl.Stdout.Warn(vbl.Id("vid/38ce8360ea6d"), err)
			return err
		}

		// Execute the critical section
		if err := criticalSection(&sc); err != nil {
			vbl.Stdout.Warn(vbl.Id("vid/9f9239102c8f"), err)
			return err
		}

		// Commit the transaction
		if err := sc.CommitTransaction(sc); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/1ed021e17ab0"), err)
			return err
		}
		// //

		return nil

	}); err != nil {
		vbl.Stdout.Warn(vbl.Id("vid/bf2741b6a829"), err)

		if err := session.AbortTransaction(context.Background()); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/dacc9c25d886"), err)
		}
		return err
	}

	vbl.Stdout.Debug(
		vbl.Id("vid/56fd66d7c9a7"),
		"Critical section completed successfully.",
	)
	return nil
}
