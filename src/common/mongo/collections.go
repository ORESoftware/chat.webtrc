// source file path: ./src/common/mongo/collections.go
package virl_mongo

import (
	"context"
	"fmt"
	virl_conf "github.com/oresoftware/chat.webtrc/src/config"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
)

const (
	VibeChatUser           = "vibe_chat_user"
	VibeChatConv           = "vibe_chat_conv"
	VibeChatConvUsers      = "vibe_chat_conv_users"
	VibeChatConvMessages   = "vibe_chat_conv_message"
	VibeChatConvMessageAck = "vibe_chat_conv_message_ack"
	VibeChatSequences      = "vibe_chat_sequences"
	VibeUserDevices        = "vibe_user_devices"
	VibeChatConvEvents     = "vibe_chat_conv_events"
	VibeMessageFile        = "vibe_message_file"
)

var Collections = struct {
	VibeChatUser           string
	VibeChatConv           string
	VibeChatConvUsers      string
	VibeChatConvMessages   string
	VibeChatConvMessageAck string
	VibeChatSequences      string
	VibeUserDevices        string
	VibeChatConvEvents     string
}{
	VibeChatUser:           VibeChatUser,
	VibeChatConv:           VibeChatConv,
	VibeChatConvUsers:      VibeChatConvUsers,
	VibeChatConvMessages:   VibeChatConvMessages,
	VibeChatConvMessageAck: VibeChatConvMessageAck,
	VibeChatSequences:      VibeChatSequences,
	VibeUserDevices:        VibeUserDevices,
	VibeChatConvEvents:     VibeChatConvEvents,
}

var cnfg = virl_conf.GetConf()

var defaultCollections = []struct {
	Name string
	DB   string
}{
	{
		Name: VibeChatUser,
		DB:   cnfg.MONGO_DB_NAME,
	},
	{
		Name: VibeChatConv,
		DB:   cnfg.MONGO_DB_NAME,
	},
	{
		Name: VibeChatConvUsers,
		DB:   cnfg.MONGO_DB_NAME,
	},
	{
		Name: VibeChatConvMessages,
		DB:   cnfg.MONGO_DB_NAME,
	},
	{
		Name: VibeChatSequences,
		DB:   cnfg.MONGO_DB_NAME,
	},
	{
		Name: VibeChatConvEvents,
		DB:   cnfg.MONGO_DB_NAME,
	},
}

var mongoCollections = map[string]*mongo.Collection{}

func AddCollectionRef(client mongo.Client, db string, collectionName string) {
	mongoCollections[collectionName] = client.Database(db).Collection(collectionName)
}

func RemoveCollectionRef(client mongo.Client, name string, db string, collectionName string) {
	delete(mongoCollections, name)
}

func AddDefaultCollections(client mongo.Client) error {
	for _, v := range defaultCollections {
		AddCollectionRef(client, v.DB, v.Name)
	}
	return nil
}

func RefCollections(db *mongo.Database) {

}

func createCollection(db *mongo.Database, collectionName string, jsonSchema bson.M, valOpts *options.CreateCollectionOptions) {

	validationOptions := valOpts.SetValidator(bson.M{"$jsonSchema": jsonSchema})

	// TODO: aggregate mongo commands
	// Apply the validation to a new or existing collection
	err := db.CreateCollection(context.TODO(), collectionName, validationOptions)
	if err != nil {
		log.Fatal("00171bfa-1de4-44c7-8959-2ea523ede76a", err)
	}

	fmt.Println(fmt.Sprintf("Successfully created collection '%s'", collectionName))
}

func CreateVibeMongoCollections(db *mongo.Database) {

	{

		collectionName := VibeChatConvMessages
		jsonSchema := bson.M{
			"bsonType": "object",
			"required": []string{"userId", "convId"},
			"properties": bson.M{
				"userId": bson.M{
					"bsonType":    "objectId",
					"description": "must be a valid non-null ObjectId",
				},
				"convId": bson.M{
					"bsonType":    "objectId",
					"description": "must be a valid non-null ObjectId",
				},
			},
		}

		validationOptions := options.CreateCollection().
			SetEncryptedFields("Messages").
			SetExpireAfterSeconds(24 * 60 * 60 * 40)

		createCollection(db, collectionName, jsonSchema, validationOptions)

	}

	{

		collectionName := VibeUserDevices
		jsonSchema := bson.M{}

		validationOptions := options.CreateCollection().
			SetEncryptedFields("DeviceId").
			SetEncryptedFields("IpAddress").
			SetExpireAfterSeconds(-1)

		createCollection(db, collectionName, jsonSchema, validationOptions)

	}

	{

		collectionName := VibeChatConvUsers
		jsonSchema := bson.M{}

		validationOptions := options.CreateCollection().
			SetExpireAfterSeconds(-1)

		createCollection(db, collectionName, jsonSchema, validationOptions)

	}

	{

		collectionName := VibeChatConv
		jsonSchema := bson.M{}

		validationOptions := options.CreateCollection().
			SetEncryptedFields("ChatTitle").
			SetExpireAfterSeconds(-1)

		createCollection(db, collectionName, jsonSchema, validationOptions)

	}

	{

		collectionName := VibeChatConvMessageAck
		jsonSchema := bson.M{}

		validationOptions := options.CreateCollection().
			SetExpireAfterSeconds(24 * 60 * 60 * 50)

		createCollection(db, collectionName, jsonSchema, validationOptions)

	}

	{

		collectionName := VibeMessageFile
		jsonSchema := bson.M{}

		validationOptions := options.CreateCollection().
			SetEncryptedFields("Url").
			SetExpireAfterSeconds(24 * 60 * 60 * 50)

		createCollection(db, collectionName, jsonSchema, validationOptions)

	}

	{

		collectionName := VibeChatUser
		jsonSchema := bson.M{}

		validationOptions := options.CreateCollection().
			SetEncryptedFields("Email").
			SetExpireAfterSeconds(-1)

		createCollection(db, collectionName, jsonSchema, validationOptions)

	}

	{

		collectionName := VibeChatSequences
		jsonSchema := bson.M{}

		validationOptions := options.CreateCollection().
			SetEncryptedFields("Email").
			SetExpireAfterSeconds(-1)

		createCollection(db, collectionName, jsonSchema, validationOptions)

	}

}
