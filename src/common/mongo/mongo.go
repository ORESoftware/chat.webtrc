// source file path: ./src/common/mongo/mongo.go
package virl_mongo

import (
	"context"
	"errors"
	"fmt"
	"github.com/oresoftware/chat.webtrc/src/common/apm"
	runtime_validation "github.com/oresoftware/chat.webtrc/src/common/mongo/runtime-validation"
	mngo_types "github.com/oresoftware/chat.webtrc/src/common/mongo/types"
	"github.com/oresoftware/chat.webtrc/src/common/mongo/vctx"
	vutils "github.com/oresoftware/chat.webtrc/src/common/v-utils"
	"github.com/oresoftware/chat.webtrc/src/common/vibelog"
	cfg "github.com/oresoftware/chat.webtrc/src/config"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

var conf = cfg.GetConf()

type Colls struct {
	ChatUser           *mongo.Collection
	ChatConv           *mongo.Collection
	ChatConvUsers      *mongo.Collection
	ChatConvMessages   *mongo.Collection
	ChatConvMessageAck *mongo.Collection
	ChatSequences      *mongo.Collection
	ChatUserDevices    *mongo.Collection
	ChatConvEvents     *mongo.Collection
	ChatMessageFile    *mongo.Collection
}

type M struct {
	DatabaseName string
	Conn         *mongo.Client
	DB           *mongo.Database
	Col          *Colls
}

func getCols(db *mongo.Database) *Colls {

	opts := options.Collection().SetBSONOptions(&options.BSONOptions{
		UseJSONStructTags:       true, // this is the only non-default value
		ErrorOnInlineDuplicates: false,
		IntMinSize:              false,
		NilMapAsEmpty:           false,
		NilSliceAsEmpty:         false,
		NilByteSliceAsEmpty:     false,
		OmitZeroStruct:          false,
		StringifyMapKeysWithFmt: false,
		AllowTruncatingDoubles:  false,
		BinaryAsSlice:           false,
		DefaultDocumentD:        false,
		DefaultDocumentM:        false,
		UseLocalTimeZone:        false,
		ZeroMaps:                false,
		ZeroStructs:             false,
	})

	return &Colls{
		ChatUser:           db.Collection(VibeChatUser, opts),
		ChatConv:           db.Collection(VibeChatConv, opts),
		ChatConvUsers:      db.Collection(VibeChatConvUsers, opts),
		ChatConvMessages:   db.Collection(VibeChatConvMessages, opts),
		ChatConvMessageAck: db.Collection(VibeChatConvMessageAck, opts),
		ChatSequences:      db.Collection(VibeChatSequences, opts),
		ChatUserDevices:    db.Collection(VibeUserDevices, opts),
		ChatConvEvents:     db.Collection(VibeChatConvEvents, opts),
		ChatMessageFile:    db.Collection(VibeMessageFile, opts),
	}
}

func NewVibeMongo(dbName string, conn *mongo.Client, db *mongo.Database) *M {
	return &M{
		DatabaseName: dbName,
		Conn:         conn,
		DB:           db,
		Col:          getCols(db),
	}
}

func (m *M) DoUpdateMaybe(v mngo_types.HasId, coll mongo.Collection) (*mongo.UpdateResult, error) {
	var id = v.GetId()
	x, err := coll.UpdateOne(bgCtx, bson.M{"_id": id}, v)
	return x, err
}

func (m *M) IncrementSequence(ctx context.Context, seqName string) (int64, error) {

	var coll = m.Col.ChatSequences
	var result bson.M
	filter := bson.M{"SeqName": seqName}
	update := bson.M{"$inc": bson.M{"SeqValue": 1}}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After) // Return the updated document

	err := coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)

	if err != nil {
		vbl.Stdout.Error(vbl.Id("vid/9f753d6e973f"), err)
		return -1, err
	}
	// result now contains the updated document
	// You can extract the sequence value like this:
	sequenceValue, ok := result["SeqValue"].(int32) // or int64 depending on your sequence value size

	if !ok {
		err := fmt.Errorf("could not retrieve int - %v", result)
		vapm.SendTrace("0b51dc0d-b093-49e1-9755-7ccbc0226feb", err)
		return -1, err
	}

	seqVal := int64(sequenceValue)
	return seqVal, nil
}

func (m *M) FindOneAndUpdate(coll *mongo.Collection, data interface{}, filter bson.M, update bson.M) (bson.M, error) {

	var result bson.M
	// filter := bson.M{"_id": "sequence_name"}
	// update := bson.M{"$inc": bson.M{"sequence_value": 1}}
	o := options.FindOneAndUpdate().SetReturnDocument(options.After) // Return the updated document

	err := coll.FindOneAndUpdate(bgCtx, filter, update, o).Decode(&result)
	return result, err

}

func (m *M) DoInsertOne(vc *vctx.MgoCtx, coll *mongo.Collection, data interface{}) (*mongo.InsertOneResult, error) {

	// for transactions, ctx will be a mongo.SessionContext instance

	defer func() {
		vc.DoCancel()
	}()

	if b := runtime_validation.ValidateBeforeInsert(data); len(b) > 0 {
		err := vutils.ErrorFromArgsList("583e21fc-eaf4-4b4c-9513-f7e68772f61f", b)
		vbl.Stdout.Warn(vbl.Id("vid/d8303e685a36"), err)
		return nil, err
	}

	if x, ok := data.(struct{ Id primitive.ObjectID }); ok {
		if x.Id.IsZero() {
			x.Id = primitive.NewObjectID()
		}
	}

	// Insert the struct into MongoDB
	v, err := coll.InsertOne(vc.GetCtx(), data)

	if err != nil {
		vbl.Stdout.Warn(vbl.Id("vid/c7ffbff533b4"), "Error inserting document:", err)
		return nil, err
	}

	return v, nil
}

func (m *M) DoUpdateOne(vc *vctx.MgoCtx, coll *mongo.Collection, id *primitive.ObjectID, x bson.M) (*mongo.UpdateResult, error) {

	defer func() {
		vc.DoCancel()
	}()

	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": x,
	}

	updateOpts := options.Update()
	result, err := coll.UpdateOne(vc.GetCtx(), filter, update, updateOpts)
	if err != nil {
		vapm.SendTrace("eb89c0f0-3fef-46ff-8ce3-0330ef5071f2", err)
		vbl.Stdout.Error(vbl.Id("vid/0434078e2b99"), err)
		return nil, err
	}

	if result.UpsertedCount > 0 {
		vbl.Stdout.InfoF("Inserted a new document with ID '%v'", result.UpsertedID)
	} else {
		vbl.Stdout.InfoF("Matched %v documents and updated %v documents.", result.MatchedCount, result.ModifiedCount)
	}

	if result.ModifiedCount < 1 {
		if vc.AllowNoMatches != true {
			vbl.Stdout.Warn(vbl.Id("vid/78fec72f426b"), "no matching documents")
			return result, fmt.Errorf("no matching documents: 'errid:%s'", "993a15a8-2975-401b-9d58-f0cf3f997269")
		}
	}

	return result, nil
}

func (m *M) DoUpsert(vc *vctx.MgoCtx, coll *mongo.Collection, filter bson.M, x bson.M) (*mongo.UpdateResult, error) {

	update := bson.M{
		"$set": x,
	}

	updateOpts := options.Update().SetUpsert(true)

	result, err := coll.UpdateOne(vc.GetCtx(), filter, update, updateOpts)
	if err != nil {
		vbl.Stdout.Warn(vbl.Id("vid/c3f323a5922b"), err)
		return nil, err
	}

	if result.UpsertedCount > 0 {
		vbl.Stdout.InfoF("Inserted a new document with ID %v", result.UpsertedID)
	} else {
		vbl.Stdout.InfoF("Matched %v documents and updated %v documents.", result.MatchedCount, result.ModifiedCount)
	}

	return result, nil
}

func (m *M) DoUpsertById(vc *vctx.MgoCtx, coll *mongo.Collection, id *primitive.ObjectID, x bson.M) (*mongo.UpdateResult, error) {

	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": x,
	}

	updateOpts := options.Update().SetUpsert(true)

	result, err := coll.UpdateOne(vc.GetCtx(), filter, update, updateOpts)
	if err != nil {
		vbl.Stdout.Warn(vbl.Id("vid/8207ad8c9952"), err)
		return nil, err
	}

	if result.UpsertedCount > 0 {
		vbl.Stdout.InfoF("Inserted a new document with ID %v", result.UpsertedID)
	} else {
		vbl.Stdout.InfoF("Matched %v documents and updated %v documents.", result.MatchedCount, result.ModifiedCount)
	}

	return result, nil
}

func (s *M) DoTrx(to int, retryCount int, maxRetries int, criticalSection func(sessionContext *mongo.SessionContext) error) error {

	if retryCount > maxRetries {
		return vutils.ErrorFromArgs("209cdb4b-7995-4c87-98ce-16fda40f83be", "too many retries for mongo transaction")
	}

	if err := DoTrx(to, s.Conn, criticalSection); err != nil {
		var transientErr *mongo.CommandError // TODO: transient error is a type of commanderror, but...
		if errors.As(err, &transientErr) {
			vbl.Stdout.Error(vbl.Id("vid/3cdd2e6abbce"), fmt.Sprintf("Transient error on attempt: %v", transientErr))
			time.Sleep(time.Second * 3)
			return s.DoTrx(to, retryCount+1, maxRetries, criticalSection) // Retry the transaction
		}
		return err
	}
	return nil
}

func (s *M) MustFindOne(
	vc *vctx.MgoCtx,
	coll *mongo.Collection,
	filter bson.M,
	d ...*options.FindOptions,
) (*bson.Raw, error) {

	defer func() {
		vc.DoCancel()
	}()

	var lim int64 = 3 // use 3 instead of 2 to help us recognize patterns and therefore debug - worth it
	z := append(d, &options.FindOptions{
		Limit: &lim,
	})

	var results []*bson.Raw
	cur, err := coll.Find(vc.GetCtx(), filter, z...)

	if err != nil {
		vapm.SendTrace("334e1a12-341a-42d7-9d2a-dcb6c8f66584", err)
		vbl.Stdout.Error(vbl.Id("vid/c01581aa6e78"), err)
		return nil, err
	}

	for cur.Next(bgCtx) {
		results = append(results, &cur.Current)
	}

	if len(results) > 1 {
		vbl.Stdout.Error(vbl.Id("vid/de2b441e02bd"), fmt.Sprintf("no matching results: %v", filter))
		return nil, fmt.Errorf("Too many matching results!! 'errid:%s'", "664815e5-1bd3-42d6-a4e3-9e58453ba6ea")
	}

	if len(results) < 1 {
		if vc.AllowNoMatches {
			return nil, nil
		}
		vbl.Stdout.Error(vbl.Id("vid/492041f086d8"), "no matching results:", filter)
		return nil, fmt.Errorf("No/zero matching results!! 'errid:%s'", "b4b30847-d15e-4cd4-802c-758e280a91d1")
	}

	if results[0] == nil {
		vbl.Stdout.Error(vbl.Id("vid/577adb8f2eae"), "nil first element")
		return nil, fmt.Errorf("strange nil value in array errorid:'%s'", "f31158b0-4003-46a6-980e-371002a22213")
	}

	return results[0], nil
}

func (s *M) FindOne(
	vc *vctx.MgoCtx,
	coll *mongo.Collection,
	filter bson.M,
	d []*options.FindOneOptions,
	to time.Duration,
) (*bson.M, error) {

	defer func() {
		vc.DoCancel()
	}()

	var result *bson.M

	if v := coll.FindOne(vc.GetCtx(), filter, d...); v != nil {
		err := v.Err()
		if err != nil {
			vapm.SendTrace("894861c1-c33c-4b0e-94c9-4c6168193013", err)
			vbl.Stdout.Error(vbl.Id("vid/06ac041da760"), err)
			return nil, err
		}
		if err := v.Decode(&result); err != nil {
			vapm.SendTrace("ee703cb9-2e16-435a-91a6-ed9eac257031", err)
			vbl.Stdout.Error(vbl.Id("vid/e916f7a3ee20"), err)
			return nil, err
		} else {
			return result, nil
		}
	}

	if vc.AllowNoMatches {
		return nil, nil
	}

	return nil, fmt.Errorf("59afc630-2766-4fdf-b39a-4651dd666034: %s", "findone result returned nil.")

}

func (s *M) FindManyCursor(
	vc *vctx.MgoCtx,
	coll *mongo.Collection,
	filter bson.M,
	d ...*options.FindOptions,
) (*mongo.Cursor, error) {

	defer func() {
		vc.DoCancel()
	}()

	cursor, err := coll.Find(vc.GetCtx(), filter, d...)

	if err != nil {
		vapm.SendTrace("08a160ec-c167-480f-b0a2-5d5b59d516c5", err)
		vbl.Stdout.Error(vbl.Id("vid/191109ce2c5e"), err)
		return nil, err
	}

	if cursor.Err() != nil {
		vapm.SendTrace("89c2bea6-5b91-4c09-a364-66ddd089da9b", err)
		vbl.Stdout.Error(vbl.Id("vid/22ed64d19fee"), err)
	}

	return cursor, nil
}

func (s *M) FindMany(
	vc *vctx.MgoCtx,
	coll *mongo.Collection,
	filter bson.M,
	d *options.FindOptions,
	to time.Duration,
) (*[]bson.M, error) {

	defer func() {
		vc.DoCancel()
	}()

	cursor, err := coll.Find(vc.GetCtx(), filter, d)

	if err != nil {
		vapm.SendTrace("589f18a1-3045-466b-84f1-9a819b70ebe9", err)
		vbl.Stdout.Error(vbl.Id("vid/763b20393f29"), err)
		return nil, err
	}

	var results []bson.M

	for cursor.Next(context.Background()) {
		var result bson.M
		err := cursor.Decode(&result)
		if err != nil {
			vapm.SendTrace("38d78932-7ac7-494e-8cc8-5e0c609bd650", err)
			vbl.Stdout.Error(vbl.Id("vid/f2c5714e29b9"), err)
			continue
		}
		results = append(results, result)
	}

	if len(results) < 1 {
		if !vc.AllowNoMatches {
			return &results, fmt.Errorf("no results returned but allowNoMatches not set to true.")
		}
	}

	return &results, nil
}

func (s *M) queryMongoCollection(vc *vctx.MgoCtx, coll *mongo.Collection, filter bson.M, d *options.FindOptions) []interface{} {

	// OLD CODE, delete soon
	results := []interface{}{}

	defer func() {
		vc.DoCancel()
	}()

	cursor, err := coll.Find(vc.GetCtx(), filter, d)

	if err != nil {
		vbl.Stdout.Error(vbl.Id("vid/00c4270e4b1a"), err)
	}

	defer func() {
		if err := cursor.Close(context.Background()); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/1d150154a9a2"), err)
		}
	}()

	for cursor.Next(context.Background()) {
		var result bson.M
		if err := cursor.Decode(&result); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/6b4829a3d7f1"), err)
		}
		// Process your result (for example, print it)
		results = append(results, result)
	}

	if err := cursor.Err(); err != nil {
		vapm.SendTrace("5529bf19-2ae2-46ad-8854-0117093492cc", err)
		vbl.Stdout.Error(vbl.Id("vid/98d1e246adcb"), err)
	}

	return results
}

func (s *M) CursorIterator(
	vc *vctx.MgoCtx,
	coll *mongo.Collection,
	filter bson.M,
	d *options.FindOptions,
) <-chan bson.M {

	ch := make(chan bson.M, 1)

	// use it like this:
	// for val := range CursorIterator(5) {
	//	fmt.Println(val)
	// }

	go func() {

		defer func() {
			vc.DoCancel()
		}()

		moreOpts := options.Find().SetLimit(5000).SetBatchSize(100)

		cursor, err := coll.Find(vc.GetCtx(), filter, d, moreOpts)

		if err != nil {
			vbl.Stdout.Error(vbl.Id("vid/bbe8c47ce857"), err)
			return
		}

		defer func() {
			if err := cursor.Close(context.Background()); err != nil {
				vbl.Stdout.Warn(vbl.Id("vid/daf18c46e4b5"), err)
			}
			close(ch) // close channel after iteration is done
		}()

		if err := cursor.Err(); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/23c701744f24"), err)
		}

		for cursor.Next(context.Background()) {
			var result bson.M
			err := cursor.Decode(&result)
			if err != nil {
				vbl.Stdout.Error(vbl.Id("vid/18b1dd1a3239"), err)
				continue
			}
			ch <- result // send value to channel
		}

		if err := cursor.Err(); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/05b123d2f95a"), err)
		}

	}()

	return ch
}

func (s *M) CursorCallbackByRow(
	vc *vctx.MgoCtx,
	coll *mongo.Collection,
	filter bson.M,
	d *options.FindOptions,
	// errBack func(error),
	cb func(*bson.M)) {

	defer func() {
		vc.DoCancel()
	}()

	moreOpts := options.Find().SetLimit(5000).SetBatchSize(100)
	cursor, err := coll.Find(vc.GetCtx(), filter, d, moreOpts)

	if err != nil {
		vbl.Stdout.Error(vbl.Id("vid/cf5dee4e42e0"), err)
		return
	}

	defer func() {
		if err := cursor.Close(context.Background()); err != nil {
			vbl.Stdout.Warn(vbl.Id("vid/fa08110b5dff"), err)
		}
	}()

	if err := cursor.Err(); err != nil {
		vbl.Stdout.Error(vbl.Id("vid/2a5210103f2e"), err)
		return
	}

	for cursor.Next(context.Background()) {
		var result *bson.M
		err := cursor.Decode(&result)
		if err != nil {
			vbl.Stdout.Error(vbl.Id("vid/403b81ecece9"), err)
			continue
		}
		// fire callback for each row
		vbl.Stdout.Debug(vbl.Id("vid/11db386be8b1"), "calling back new doc:", result)
		cb(result)
	}

	if err := cursor.Err(); err != nil {
		vbl.Stdout.Error(vbl.Id("vid/6dce601327f7"), err)
	}
}
