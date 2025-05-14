// source file path: ./src/common/mongo/indexes.go
package virl_mongo

import (
	vutils "github.com/oresoftware/chat.webrtc/src/common/v-utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"sync"
)

var once = sync.Once{}

type IndexPair struct {
	Index      *mongo.IndexModel
	Collection *mongo.Collection
}

func CreateIndexes(dbName string, db *mongo.Database) {

	if db.Name() != dbName {
		panic(vutils.ErrorFromArgs("09d4ed29-faca-448b-9007-9433057c8874", "mis-match in db names"))
	}

	once.Do(func() {
		var indexes = getIndexes(dbName, db)
		for _, idx := range indexes {
			_, err := idx.Collection.Indexes().CreateOne(bgCtx, *idx.Index)
			if err != nil {
				log.Fatal("3d2bc9ee-8671-46ed-89fc-93c866e541ce", err)
			}
		}
	})
}

func getIndexes(dbName string, db *mongo.Database) []IndexPair {

	if db.Name() != dbName {
		panic(vutils.ErrorFromArgs("0db40d68-6b93-45ff-9229-d26c2ce47c8a", "mis-match in db names"))
	}

	return []IndexPair{
		{
			Collection: db.Collection(VibeChatConvMessageAck),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"ChatId": 1,
				},
				Options: options.Index().SetUnique(false), // additional index options like uniqueness
			},
		},

		{
			Collection: db.Collection(VibeChatConvMessageAck),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"MessageId": 1, // specify the field name and order (1 for ascending, -1 for descending)
				},
				Options: options.Index().SetUnique(false), // additional index options like uniqueness
			},
		},

		{
			Collection: db.Collection(VibeChatConvMessageAck),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"UserId": 1, // specify the field name and order (1 for ascending, -1 for descending)
				},
				Options: options.Index().SetUnique(false), // additional index options like uniqueness
			},
		},

		{
			Collection: db.Collection(VibeChatConvMessageAck),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"ChatId":    1,
					"MessageId": 1, // specify the field name and order (1 for ascending, -1 for descending)
				},
				Options: options.Index().SetUnique(false), // additional index options like uniqueness
			},
		},

		{
			Collection: db.Collection(VibeChatConvMessageAck),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"ChatId":    1,
					"UserId":    1,
					"MessageId": 1, // specify the field name and order (1 for ascending, -1 for descending)
				},
				Options: options.Index().SetUnique(false), // additional index options like uniqueness
			},
		},

		{
			Collection: db.Collection(VibeChatSequences),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"SeqName": 1,
				},
				Options: options.Index().SetUnique(true), // additional index options like uniqueness
			},
		},

		{
			Collection: db.Collection(VibeUserDevices),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"UserId": 1,
				},
				Options: options.Index().SetUnique(false), // additional index options like uniqueness
			},
		},

		{
			Collection: db.Collection(VibeUserDevices),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"UserId": 1,
					"SeqNum": 1, // specify the field name and order (1 for ascending, -1 for descending)
				},
				Options: options.Index().SetUnique(true), // additional index options like uniqueness
			},
		},

		{
			Collection: db.Collection(VibeUserDevices),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"UserId":   1,
					"DeviceId": 1, // specify the field name and order (1 for ascending, -1 for descending)
				},
				Options: options.Index().SetUnique(true), // additional index options like uniqueness
			},
		},

		{
			Collection: db.Collection(VibeChatConvUsers),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"UserId": 1, // specify the field name and order (1 for ascending, -1 for descending)
				},
				Options: options.Index().SetUnique(false), // additional index options like uniqueness
			},
		},

		{
			Collection: db.Collection(VibeChatConvUsers),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"ChatId": 1, // specify the field name and order (1 for ascending, -1 for descending)
				},
				Options: options.Index().SetUnique(false), // additional index options like uniqueness
			},
		},

		{
			Collection: db.Collection(VibeChatConvUsers),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"ChatId": 1, // specify the field name and order (1 for ascending, -1 for descending)
					"UserId": 1, // specify the field name and order (1 for ascending, -1 for descending)
				},
				Options: options.Index().SetUnique(false), // additional index options like uniqueness
			},
		},

		{
			Collection: db.Collection(VibeChatUser),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"Handle": 1, // specify the field name and order (1 for ascending, -1 for descending)
				},
				Options: options.Index().SetUnique(true), // additional index options like uniqueness
			},
		},

		// TODO: what if the order (asc vs desc) doesn't matter?

		{
			Collection: db.Collection(VibeChatUser),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"CreatedBy": 1, // specify the field name and order (1 for ascending, -1 for descending)
				},
				Options: options.Index().SetUnique(false), // additional index options like uniqueness
			},
		},

		{
			Collection: db.Collection(VibeChatConv),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"ChatId": 1, // specify the field name and order (1 for ascending, -1 for descending)
				},
				Options: options.Index().SetUnique(true), // additional index options like uniqueness
			},
		},

		{
			// expire messages after 15 weeks
			Collection: db.Collection(VibeChatConvMessages),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"CreatedAt": 1, // specify the field name and order (1 for ascending, -1 for descending)
				},
				// 15 weeks of seconds = 9072000 seconds
				Options: options.Index().SetUnique(false).SetExpireAfterSeconds(9072000),
			},
		},

		{
			Collection: db.Collection(VibeChatConvMessages),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"CreatedByUserId": 1, // specify the field name and order (1 for ascending, -1 for descending)
				},
				Options: options.Index().SetUnique(false), // additional index options like uniqueness
			},
		},

		{
			Collection: db.Collection(VibeChatConvMessages),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"ChatId": 1, // specify the field name and order (1 for ascending, -1 for descending)
				},
				Options: options.Index().SetUnique(false), // additional index options like uniqueness
			},
		},

		{
			Collection: db.Collection(VibeChatConvMessages),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"DateCreatedOnDevice": 1, // specify the field name and order (1 for ascending, -1 for descending)
					"ChatId":              1, // specify the field name and order (1 for ascending, -1 for descending)
				},
				Options: options.Index().SetUnique(false), // additional index options like uniqueness
			},
		},

		{
			Collection: db.Collection(VibeChatConvMessages),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"UserId":              1, // specify the field name and order (1 for ascending, -1 for descending)
					"DateCreatedOnDevice": 1, // specify the field name and order (1 for ascending, -1 for descending)
					"ChatId":              1, // specify the field name and order (1 for ascending, -1 for descending)
				},
				Options: options.Index().SetUnique(true), // additional index options like uniqueness
			},
		},

		{
			Collection: db.Collection(VibeChatConvMessages),
			Index: &mongo.IndexModel{
				Keys: bson.M{
					"UserId":              1, // specify the field name and order (1 for ascending, -1 for descending)
					"DateCreatedOnDevice": 1, // specify the field name and order (1 for ascending, -1 for descending)
					"ChatId":              1, // specify the field name and order (1 for ascending, -1 for descending)
				},
				Options: options.Index().SetUnique(true), // additional index options like uniqueness
			},
		},
	}

}
