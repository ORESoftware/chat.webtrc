// source file path: ./src/common/mongo/schema/schema-generate.go
package main

import (
	"encoding/json"
	"fmt"
	"github.com/invopop/jsonschema"
	mngo_types "github.com/oresoftware/chat.webrtc/src/common/mongo/types"
	vbu "github.com/oresoftware/chat.webrtc/src/common/v-utils"
	"go.mongodb.org/mongo-driver/bson"
	"os"
)

// Define your struct here. This should mirror your JSON data.

type MyData1 struct {
	Name    string `json:"name"`
	Age     int    `json:"age"`
	Address string `json:"address,omitempty"`
}

type container[T any] struct {
	myType T
	caster func() T
}

var Types = []container[mngo_types.MongoMessageAck]{
	container[mngo_types.MongoMessageAck]{},
}

var generateSchemaStr = func(s *jsonschema.Schema, fileName string) string {
	// Marshal the schema into JSON
	schemaJSON, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		panic(vbu.ErrorFromArgs("03348543-a5a2-4b93-a332-8d11b3b7c451", err.Error()))
	}

	filePath := fmt.Sprintf("%s/%s/%s", os.Getenv("PWD"), "src/common/mongo/schema/values", fileName)
	// file, err := os.OpenFile(filePath, 1, os.ModeAppend)
	file, err := os.Create(filePath)

	if err != nil {
		panic(vbu.ErrorFromArgs("d0bbd957-70dd-4862-b673-b40c079da3e4", err.Error()))
	}

	fmt.Fprintln(file, string(schemaJSON))

	return filePath
}

func RunThis() {
	// Generate a JSON schema for your struct

	{
		schema := jsonschema.Reflect(&mngo_types.MongoChatUser{})
		fileName := "MongoChatUser.schema.json"
		filePath := generateSchemaStr(schema, fileName)
		fmt.Println(filePath)
	}

	{
		schema := jsonschema.Reflect(&mngo_types.MongoMessageAck{})
		fileName := "MongoMessageAck.schema.json"
		filePath := generateSchemaStr(schema, fileName)
		fmt.Println(filePath)
	}

	{
		schema := jsonschema.Reflect(&mngo_types.MongoChatMessage{})
		fileName := "MongoChatMessage.schema.json"
		filePath := generateSchemaStr(schema, fileName)
		fmt.Println(filePath)
	}

	{
		schema := jsonschema.Reflect(&mngo_types.MongoChatUserDevice{})
		fileName := "MongoChatUserDevice.schema.json"
		filePath := generateSchemaStr(schema, fileName)
		fmt.Println(filePath)
	}

	{
		schema := jsonschema.Reflect(&mngo_types.MongoChatMapToUser{})
		fileName := "MongoChatMapToUser.schema.json"
		filePath := generateSchemaStr(schema, fileName)
		fmt.Println(filePath)
	}

	{
		schema := jsonschema.Reflect(&mngo_types.MongoChatConv{})
		fileName := "MongoChatConv.schema.json"
		filePath := generateSchemaStr(schema, fileName)
		fmt.Println(filePath)
	}

	{
		var MyData = bson.M{
			"this_is_string": "value",
			"this_is_int":    5,
			"this_is_struct": MyData1{
				Name:    "<name?",
				Age:     0,
				Address: "<address>",
			},
		}
		schema := jsonschema.Reflect(&MyData)

		// Marshal the schema into JSON
		schemaJSON, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			panic(vbu.ErrorFromArgs("3314999f-61cf-4785-a643-3d34a40bfc64", (err).Error()))
		}

		// Print the JSON schema
		fmt.Println(string(schemaJSON))
	}
}

func main() {
	RunThis()
}
