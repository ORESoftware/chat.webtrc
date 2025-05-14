// source file path: ./src/common/mongo/transaction-2.go
package virl_mongo

import (
	"context"
	vbl "github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"log"
	"time"
)

func DoTrx1() {
	// Connect to MongoDB
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("your_mongodb_uri"))
	if err != nil {
		log.Fatal(vbl.Id("vid/a76f3744f3c2"), err)
	}

	// Start a session
	session, err := client.StartSession()
	if err != nil {
		log.Fatal(vbl.Id("vid/015c7a401bb7"), err)
	}
	defer session.EndSession(context.Background())

	// Start a transaction
	err = session.StartTransaction()
	if err != nil {
		log.Fatal(vbl.Id("vid/08bc70b6cda3"), err)
	}

	// Define the operations inside the transaction
	err = mongo.WithSession(context.Background(), session, func(sc mongo.SessionContext) error {
		// Perform your CRUD operations here

		// If operations are successful, commit the transaction
		if err := session.CommitTransaction(sc); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		// If there's an error, abort the transaction
		session.AbortTransaction(context.Background())
		log.Fatal(vbl.Id("vid/d6645bd2c899"), err)
	}
}

func DoTrx2(client *mongo.Client, criticalSection func(sessionContext *mongo.SessionContext) error) (interface{}, error) {
	// MongoDB connection

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer func() {
		cancel()
	}()

	// Create a session to start a transaction
	session, err := client.StartSession()

	if err != nil {
		vbl.Stdout.Warn(vbl.Id("vid/e5d716d1349d"), err)
		return nil, err
	}

	defer session.EndSession(ctx)

	// Set up transaction options
	txnOpts := options.Transaction().
		SetReadPreference(readpref.Primary()).   // Set Read Preference
		SetWriteConcern(writeconcern.Majority()) // Set Write Concern using the updated method

	transactionBody := func(sessionContext mongo.SessionContext) (interface{}, error) {

		if err := criticalSection(&sessionContext); err != nil {
			return nil, err
		}
		return nil, nil

	}

	result, err := session.WithTransaction(bgCtx, transactionBody, txnOpts)

	if err != nil {
		vbl.Stdout.Warn(vbl.Id("vid/00021a9c8a56"), err)
	} else {
		vbl.Stdout.Debug(vbl.Id("vid/ae35aefa8da0"), "Critical section completed successfully:", result)
	}

	return result, err
}
