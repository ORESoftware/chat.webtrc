// source file path: ./src/common/mongo/cursor.go
package virl_mongo

import (
	"context"
	vbl "github.com/oresoftware/chat.webtrc/src/common/vibelog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
)

func GetCursor(client mongo.Client, collection mongo.Collection) {

	cursor, err := collection.Find(context.TODO(), bson.D{{}})
	if err != nil {
		log.Fatal(vbl.Id("vid/9a93724b640f"), err)
	}

	defer cursor.Close(context.TODO())

	for cursor.Next(context.TODO()) {
		var result bson.M
		err := cursor.Decode(&result)
		if err != nil {
			log.Fatal(vbl.Id("vid/cc6f2ba26789"), err)
		}
		// Process your result here
		log.Println(result)
	}

	if err := cursor.Err(); err != nil {
		log.Fatal(vbl.Id("vid/03c34798b960"), err)
	}

}
