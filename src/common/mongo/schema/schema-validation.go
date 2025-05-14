// source file path: ./src/common/mongo/schema/schema-validation.go
package main

import (
	"context"
	vbl "github.com/oresoftware/chat.webtrc/src/common/vibelog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"os"
)

func Run() {
	// Connect to MongoDB
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("your_mongodb_uri"))
	if err != nil {
		vbl.Stdout.Critical(vbl.Id("vid/dd0ff40a000f"), err)
		os.Exit(1)
	}

	defer client.Disconnect(context.TODO())

	// Define the JSON schema for validation
	jsonSchema := bson.M{
		"bsonType": "object",
		"required": []string{"name", "age"},
		"properties": bson.M{
			"name": bson.M{
				"bsonType":    "string",
				"minLength":   1, // Require the string to have a length of 1 or greater
				"maxLength":   1, // Require the string to have a length of 1 or greater
				"description": "must be a string and is required",
			},
			"age": bson.M{
				"bsonType":    "int",
				"minimum":     0,
				"description": "must be an integer greater than or equal to 0 and is required",
			},
		},
	}

	// Set the validation options
	validationOptions := options.CreateCollection().SetValidator(bson.M{"$jsonSchema": jsonSchema})

	// Apply the validation to a new or existing collection
	db := client.Database("your_db_name")
	err = db.CreateCollection(context.TODO(), "your_collection_name", validationOptions)
	if err != nil {
		vbl.Stdout.Critical(vbl.Id("vid/af6310bf5f37"), err)
		os.Exit(1)
	}

	// Now, any insertions or updates to the collection will have to pass the validation rules
}

func main2() {
	Run()
}
