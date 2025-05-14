// source file path: ./src/common/mongo/schema/schema-val-test.go
package main

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"testing"
)

// Person struct for testing
type Person struct {
	Name string `bson:"name"`
	Age  int    `bson:"age"`
}

func TestPersonSchemaValidation(t *testing.T) {
	// Setup MongoDB connection (use a test database)
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		t.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.TODO())

	db := client.Database("test_db")
	collectionName := "test_collection"

	// Define and create a collection with JSON Schema validation
	// (Similar to your actual schema creation code)

	// Test inserting a valid document
	validPerson := Person{Name: "John Doe", Age: 30}
	_, err = db.Collection(collectionName).InsertOne(context.Background(), validPerson)
	if err != nil {
		t.Errorf("Failed to insert valid document: %v", err)
	}

	// Test inserting an invalid document
	invalidPerson := Person{Name: "", Age: -1} // Assuming this violates the schema
	_, err = db.Collection(collectionName).InsertOne(context.Background(), invalidPerson)
	if err == nil {
		t.Errorf("Expected an error when inserting invalid document, but got none")
	}

	// Clean up (drop the test collection or database if necessary)
}

func main() {
	TestPersonSchemaValidation(&testing.T{})
}
