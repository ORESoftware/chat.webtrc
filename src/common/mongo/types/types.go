// source file path: ./src/common/mongo/types/types.go
package mngo_types

import (
	"encoding/json"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strings"
	"time"
)

type MongoDoc interface {
	PrintId() string
}

type HasId interface {
	GetId() primitive.ObjectID
	SetId()
}

func printId(m MongoDoc) {
	m.PrintId()
}

//easyjson:json
type MongoUserTags struct {
	Id         primitive.ObjectID     `bson:"_id" json:"_id"`
	UserId     primitive.ObjectID     `bson:"UserId" json:"UserId"`
	CtxList    []interface{}          `bson:"CtxList" json:"CtxList"` // a post, a message, a picture without text, etc
	CtxMap     map[string]interface{} `bson:"CtxMap" json:"CtxMap"`   // a post, a message, a picture without text, etc
	TagsList   []interface{}          `bson:"TagsList" json:"TagsList"`
	TagsMap    map[string]interface{} `bson:"TagsMap" json:"TagsMap"`
	Files      []string               `bson:"Files" json:"Files"`
	SoftDelete bool                   `bson:"SoftDelete" json:"SoftDelete"`
	CreatedBy  primitive.ObjectID     `bson:"CreatedBy" json:"CreatedBy"`
	UpdatedBy  primitive.ObjectID     `bson:"UpdatedBy" json:"UpdatedBy"`
	CreatedAt  time.Time              `bson:"CreatedAt" json:"CreatedAt"`
	UpdatedAt  time.Time              `bson:"UpdatedAt" json:"UpdatedAt"`
}

func (m *MongoUserTags) GetId() primitive.ObjectID {
	return m.Id
}

func (m *MongoUserTags) SetId(x *primitive.ObjectID) {
	if x != nil {
		m.Id = *x
	}
}

//easyjson:json
type MongoChatUserDevice struct {
	Id             primitive.ObjectID     `bson:"_id" json:"_id"`
	UserId         primitive.ObjectID     `bson:"UserId" json:"UserId"`
	SeqNum         int                    `bson:"SeqNum" json:"SeqNum"`
	DeviceName     string                 `bson:"DeviceName" json:"DeviceName"`
	DeviceId       primitive.ObjectID     `bson:"DeviceId" json:"DeviceId"`
	DeviceType     string                 `bson:"DeviceType" json:"DeviceType"`         // chrome/browser, android-app
	DevicePlatform string                 `bson:"DevicePlatform" json:"DevicePlatform"` // linux, android, ios, windows
	SoftDelete     bool                   `bson:"SoftDelete" json:"SoftDelete"`
	CreatedBy      primitive.ObjectID     `bson:"CreatedBy" json:"CreatedBy"`
	UpdatedBy      primitive.ObjectID     `bson:"UpdatedBy" json:"UpdatedBy"`
	CreatedAt      time.Time              `bson:"CreatedAt" json:"CreatedAt"`
	UpdatedAt      time.Time              `bson:"UpdatedAt" json:"UpdatedAt"`
	Tags           map[string]interface{} `bson:"Tags" json:"Tags"`
}

func (v *MongoChatUserDevice) Validate() {

}

//easyjson:json
type MongoChatUser struct {
	Id                 primitive.ObjectID     `bson:"_id" json:"_id"`
	Handle             string                 `bson:"Handle" json:"Handle"`
	FirstName          string                 `bson:"FirstName" json:"FirstName"`
	LastName           string                 `bson:"LastName" json:"LastName"`
	Email              string                 `bson:"Email" json:"Email"`
	BirthMonth         int                    `bson:"BirthMonth" json:"BirthMonth"`
	BirthDay           int                    `bson:"BirthDay" json:"BirthDay"`
	BirthYear          int                    `bson:"BirthYear" json:"BirthYear"`
	DefaultTimezone    int                    `bson:"DefaultTimezone" json:"DefaultTimezone"`
	ChatDeviceSeqValue int                    `bson:"ChatDeviceSeqValue" json:"ChatDeviceSeqValue"`
	Status             string                 `bson:"Status" json:"Status"`
	Interests          []string               `bson:"Interests" json:"Interests"`
	ProfilePictureURL  string                 `bson:"ProfilePictureURL" json:"ProfilePictureURL"`
	Bio                string                 `bson:"Bio" json:"Bio"`
	City               string                 `bson:"City" json:"City"`
	Country            string                 `bson:"Country" json:"Country"`
	Notifications      bool                   `bson:"Notifications" json:"Notifications"`
	PasswordHash       string                 `bson:"PasswordHash" json:"-"`
	AuthToken          string                 `bson:"AuthToken" json:"-"`
	LastActiveTime     time.Time              `bson:"LastActiveTime" json:"LastActiveTime"`
	Language           string                 `bson:"Language" json:"Language"`
	EmailVerified      bool                   `bson:"EmailVerified" json:"EmailVerified"`
	TwoFactorEnabled   bool                   `bson:"TwoFactorEnabled" json:"TwoFactorEnabled"`
	Theme              string                 `bson:"Theme" json:"Theme"`
	FontSize           int                    `bson:"FontSize" json:"FontSize"`
	Reputation         int                    `bson:"Reputation" json:"Reputation"`
	ShareLocation      bool                   `bson:"ShareLocation" json:"ShareLocation"`
	DoNotDisturb       bool                   `bson:"DoNotDisturb" json:"DoNotDisturb"`
	SoftDelete         bool                   `bson:"SoftDelete" json:"SoftDelete"`
	CreatedBy          primitive.ObjectID     `bson:"CreatedBy" json:"CreatedBy"`
	UpdatedBy          primitive.ObjectID     `bson:"UpdatedBy" json:"UpdatedBy"`
	CreatedAt          time.Time              `bson:"CreatedAt" json:"CreatedAt"`
	UpdatedAt          time.Time              `bson:"UpdatedAt" json:"UpdatedAt"`
	TagsMap            map[string]interface{} `bson:"TagsMap" json:"TagsMap"`
	TagsList           []interface{}          `bson:"TagsList" json:"TagsList"`

	// Activity Log
}

func (m MongoChatUser) printId() {
	m.printId()
}

//easyjson:json
type MongoChatConvUserSettings struct {
	Id           primitive.ObjectID `bson:"_id" json:"_id"`
	UserId       primitive.ObjectID `bson:"UserId" json:"UserId"`
	ConvId       primitive.ObjectID `bson:"ConvId" json:"ConvId"`
	ReadReceipts bool               `bson:"ReadReceipts" json:"ReadReceipts"`
	SoftDelete   bool               `bson:"SoftDelete" json:"SoftDelete"`
	CreatedBy    primitive.ObjectID `bson:"CreatedBy" json:"CreatedBy"`
	UpdatedBy    primitive.ObjectID `bson:"UpdatedBy" json:"UpdatedBy"`
	CreatedAt    time.Time          `bson:"CreatedAt" json:"CreatedAt"`
	UpdatedAt    time.Time          `bson:"UpdatedAt" json:"UpdatedAt"`
}

//easyjson:json
type MongoChatUserUserRelation struct {
	Id         primitive.ObjectID `bson:"_id" json:"_id"`
	UserFrom   primitive.ObjectID `bson:"UserFrom" json:"UserFrom"`
	UserTo     primitive.ObjectID `bson:"UserTo" json:"UserTo"`
	Action     string             `bson:"Action" json:"Action"` // blocked user, favorited/pinned user, etc
	SoftDelete bool               `bson:"SoftDelete" json:"SoftDelete"`
	CreatedBy  primitive.ObjectID `bson:"CreatedBy" json:"CreatedBy"`
	UpdatedBy  primitive.ObjectID `bson:"UpdatedBy" json:"UpdatedBy"`
	CreatedAt  time.Time          `bson:"CreatedAt" json:"CreatedAt"`
	UpdatedAt  time.Time          `bson:"UpdatedAt" json:"UpdatedAt"`
}

//easyjson:json
type MongoUserOnline struct {
	Id             primitive.ObjectID `bson:"_id" json:"_id"`
	UserId         primitive.ObjectID `bson:"UserId" json:"UserId"`
	DeviceId       primitive.ObjectID `bson:"DeviceId" json:"DeviceId"`
	ConnectDate    time.Time          `bson:"ConnectDate" json:"ConnectDate"`
	DisconnectDate time.Time          `bson:"DisconnectDate" json:"DisconnectDate"`
	SoftDelete     bool               `bson:"SoftDelete" json:"SoftDelete"`
}

//easyjson:json
type MongoChatUserActivityLog struct {
	Id         primitive.ObjectID     `bson:"_id" json:"_id"`
	UserId     primitive.ObjectID     `bson:"UserId" json:"UserId"`
	Message    string                 `bson:"Messages" json:"Messages"`
	Url        string                 `bson:"Url" json:"Url"`
	Type       string                 `bson:"Type" json:"Type"`
	Order      int                    `bson:"Order" json:"Order"`
	SoftDelete bool                   `bson:"SoftDelete" json:"SoftDelete"`
	CreatedBy  primitive.ObjectID     `bson:"CreatedBy" json:"CreatedBy"`
	UpdatedBy  primitive.ObjectID     `bson:"UpdatedBy" json:"UpdatedBy"`
	CreatedAt  time.Time              `bson:"CreatedAt" json:"CreatedAt"`
	UpdatedAt  time.Time              `bson:"UpdatedAt" json:"UpdatedAt"`
	Tags       map[string]interface{} `bson:"Tags" json:"Tags"`
}

//easyjson:json
type MongoMessageFile struct {
	Id         primitive.ObjectID     `bson:"_id" json:"_id"`
	Name       string                 `bson:"Name" json:"Name"`
	Message    string                 `bson:"Messages" json:"Messages"`
	Url        string                 `bson:"Url" json:"Url"`
	Type       string                 `bson:"Type" json:"Type"`
	Order      int                    `bson:"Order" json:"Order"`
	SoftDelete bool                   `bson:"SoftDelete" json:"SoftDelete"`
	CreatedBy  primitive.ObjectID     `bson:"CreatedBy" json:"CreatedBy"`
	UpdatedBy  primitive.ObjectID     `bson:"UpdatedBy" json:"UpdatedBy"`
	CreatedAt  time.Time              `bson:"CreatedAt" json:"CreatedAt"`
	UpdatedAt  time.Time              `bson:"UpdatedAt" json:"UpdatedAt"`
	Tags       map[string]interface{} `bson:"Tags" json:"Tags"`
}

//easyjson:json
type TracingInfo struct {
	ProcUUID   string
	UserId     string
	Station    string
	Time       *time.Time
	SoftDelete bool               `bson:"SoftDelete" json:"SoftDelete"`
	CreatedBy  primitive.ObjectID `bson:"CreatedBy" json:"CreatedBy"`
	UpdatedBy  primitive.ObjectID `bson:"UpdatedBy" json:"UpdatedBy"`
	CreatedAt  time.Time          `bson:"CreatedAt" json:"CreatedAt"`
	UpdatedAt  time.Time          `bson:"UpdatedAt" json:"UpdatedAt"`
	Tags       map[string]interface{}
}

// TODO put in one empty/zero UUID for each field so that Mongo rejects it, if it's empty

type ChatMessageMarker struct {
	Marker string
}

// Implement the String method for the Person struct
func (p *ChatMessageMarker) String() string {
	return `ChatMessage`
}

// Custom JSON Marshaller for MongoChatConv
func (m *ChatMessageMarker) MarshalJSON() ([]byte, error) {
	return []byte(`"ChatMessage"`), nil
}

func (m *ChatMessageMarker) UnmarshalJSON(data []byte) error {
	// Assuming the JSON value is just a plain string for the Marker.
	// You need to unquote JSON strings, as the JSON representation of a string is quoted.
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	m.Marker = m.String() // You might want to convert `str` to the expected format or value
	return nil
}

//easyjson:json
type MongoChatMessage struct {
	// note that: PriorityUserIds does not need to live in mongo, just in a payload from Client
	// * pointers are used when we want to completely omit (not have "null" in bson)
	Id                  primitive.ObjectID   `bson:"_id" json:"_id"`
	Marker              ChatMessageMarker    `bson:"-" json:"Marker"`
	SoftDelete          bool                 `bson:"SoftDelete" json:"SoftDelete"`
	CreatedByUserId     primitive.ObjectID   `bson:"CreatedByUserId" json:"CreatedByUserId"`
	ChatId              primitive.ObjectID   `bson:"ChatId" json:"ChatId"`
	PriorityUserIds     []primitive.ObjectID `bson:"-" json:"PriorityUserIds"`                     // most likely the 2 chat participants, since 99% of chats are 2 people
	PreviousMessageIds  []primitive.ObjectID `bson:"PreviousMessageIds" json:"PreviousMessageIds"` // previous message ids in order of appearance
	IsGroupChat         bool                 `bson:"IsGroupChat" json:"IsGroupChat"`               // if not a group chat, then user count is always 2, and we can optimize
	IsPublicChatRoom    bool                 `bson:"IsPublicChatRoom" json:"IsPublicChatRoom"`
	UserCount           int                  `bson:"UserCount" json:"UserCount"` // if user count = 2, then GroupChat could still be true but a few users were dropped
	Messages            []string             `bson:"Messages" json:"Messages"`
	TracingInfo         []TracingInfo        `bson:"TracingInfo" json:"TracingInfo"`
	ReplayCount         int                  `bson:"ReplayCount" json:"ReplayCount"`
	IsReaction          *bool                `bson:"IsReaction,omitempty" json:"IsReaction,omitempty"`
	IsDeleted           *bool                `bson:"IsDeleted,omitempty" json:"IsDeleted,omitempty"` // never delete from disk because you never know if it was branched with an edit etc
	DisplayFormat       *string              // like markdown, html, etc
	DateCreatedOnDevice *time.Time           `bson:"DateCreatedOnDevice,omitempty" bson:"DateCreatedOnDevice,omitempty"`
	DateFirstOnServer   *time.Time           `bson:"DateFirstOnServer,omitempty" json:"DateFirstOnServer,omitempty"`
	ChildId             *primitive.ObjectID  `bson:"ChildId,omitempty" json:"ChildId,omitempty"`
	ChildType           *string              `bson:"ChildType,omitempty" json:"ChildType,omitempty"`
	ParentId            *primitive.ObjectID  `bson:"ParentId,omitempty" json:"ParentId,omitempty"`
	ParentType          *string              `bson:"ParentType,omitempty" json:"ParentType,omitempty"`
	GrandParentId       *primitive.ObjectID  `bson:"GrandParentId,omitempty" json:"GrandParentId,omitempty"` // the id of the first message that was branched-off of main thread
	Files               *[]MongoMessageFile  `bson:"Files,omitempty" json:"Files,omitempty"`
	IsDraft             *bool                `bson:"IsDraft,omitempty" json:"IsDraft,omitempty"` //
	CreatedBy           primitive.ObjectID   `bson:"CreatedBy" json:"CreatedBy"`
	UpdatedBy           primitive.ObjectID   `bson:"UpdatedBy" json:"UpdatedBy"`
	CreatedAt           time.Time            `bson:"CreatedAt" json:"CreatedAt"`
	UpdatedAt           time.Time            `bson:"UpdatedAt" json:"UpdatedAt"`
	Tags                *map[string]interface {
	} `bson:"Tags,omitempty" json:"Tags,omitempty"`
}

func (m *MongoChatMessage) GetObjectId() primitive.ObjectID {
	return m.Id
}

func (m *MongoChatMessage) GetId() string {
	return m.Id.Hex()
}

func (m *MongoChatMessage) PrintId() {

}

type Location struct {
	Type   string
	Coords []float64
}

type UserLocationMarker string

func (*UserLocationMarker) String() string {
	return `UserLocation`
}

func (*UserLocationMarker) MarshalJSON() ([]byte, error) {
	return []byte(`"UserLocation"`), nil
}

func (amm *UserLocationMarker) UnmarshalJSON(data []byte) error {
	*amm = UserLocationMarker(strings.Trim(string(data), `"`))
	return nil
}

//easyjson:json
type MongoUserLocation struct {
	Id         primitive.ObjectID     `bson:"_id" json:"_id"`
	Marker     UserLocationMarker     `bson:"-" json:"UserLocationMarker"`
	UserId     primitive.ObjectID     `bson:"UserId" json:"UserId"`
	Location   Location               `bson:"Location" json:"Location"`
	SoftDelete bool                   `bson:"SoftDelete" json:"SoftDelete"`
	CreatedBy  primitive.ObjectID     `bson:"CreatedBy" json:"CreatedBy"`
	UpdatedBy  primitive.ObjectID     `bson:"UpdatedBy" json:"UpdatedBy"`
	CreatedAt  time.Time              `bson:"CreatedAt" json:"CreatedAt"`
	UpdatedAt  time.Time              `bson:"UpdatedAt" json:"UpdatedAt"`
	Tags       map[string]interface{} `bson:"Tags" json:"Tags"`
}

type AckMessageMarker struct {
	Marker string
}

// Implement the String method for the Person struct
func (p *AckMessageMarker) String() string {
	return `AckMessage`
}

// Custom JSON Marshaller for MongoChatConv
func (m *AckMessageMarker) MarshalJSON() ([]byte, error) {
	return []byte(`"AckMessage"`), nil
}

func (m *AckMessageMarker) UnmarshalJSON(data []byte) error {
	// Assuming the JSON value is just a plain string for the Marker.
	// You need to unquote JSON strings, as the JSON representation of a string is quoted.
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	m.Marker = m.String() // You might want to convert `str` to the expected format or value
	return nil
}

//easyjson:json
type MongoMessageAck struct {
	Id         primitive.ObjectID     `bson:"_id" json:"_id"`
	Marker     AckMessageMarker       `bson:"-" json:"Marker"`
	MessageId  primitive.ObjectID     `bson:"MessageId" json:"MessageId"`
	ChatId     primitive.ObjectID     `bson:"ChatId" json:"ChatId"`
	UserId     primitive.ObjectID     `bson:"UserId" json:"UserId"`
	IsRead     bool                   `bson:"IsRead" json:"IsRead"`
	IsSaved    bool                   `bson:"IsSaved" json:"IsSaved"` // saved to client disk
	SoftDelete bool                   `bson:"SoftDelete" json:"SoftDelete"`
	CreatedBy  primitive.ObjectID     `bson:"CreatedBy" json:"CreatedBy"`
	UpdatedBy  primitive.ObjectID     `bson:"UpdatedBy" json:"UpdatedBy"`
	CreatedAt  time.Time              `bson:"CreatedAt" json:"CreatedAt"`
	UpdatedAt  time.Time              `bson:"UpdatedAt" json:"UpdatedAt"`
	Tags       map[string]interface{} `bson:"Tags" json:"Tags"`
}

type ChatConvMarker struct {
	Marker string
}

// Implement the String method for the Person struct
func (p *ChatConvMarker) String() string {
	return `ChatConv`
}

// Custom JSON Marshaller for MongoChatConv
func (m *ChatConvMarker) MarshalJSON() ([]byte, error) {
	return []byte(`"ChatConv"`), nil
}

func (m *ChatConvMarker) UnmarshalJSON(data []byte) error {
	// Assuming the JSON value is just a plain string for the Marker.
	// You need to unquote JSON strings, as the JSON representation of a string is quoted.
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	m.Marker = m.String() // You might want to convert `str` to the expected format or value
	return nil
}

//easyjson:json
type MongoChatConv struct {
	Id           primitive.ObjectID     `bson:"_id" json:"_id"`
	Marker       ChatConvMarker         `bson:"-" json:"Marker"`
	SeqNum       int64                  `bson:"SeqNum" json:"SeqNum"`
	ChatTitle    string                 `bson:"ChatTitle" json:"ChatTitle"`
	MessageCount int                    `bson:"MessageCount" json:"MessageCount"` // should be updated by a chron-job
	PrivateKey   string                 `bson:"PrivateKey" json:"PrivateKey"`     // encrypted with previous key
	SoftDelete   bool                   `bson:"SoftDelete" json:"SoftDelete"`
	CreatedBy    primitive.ObjectID     `bson:"CreatedBy" json:"CreatedBy"`
	UpdatedBy    primitive.ObjectID     `bson:"UpdatedBy" json:"UpdatedBy"`
	CreatedAt    time.Time              `bson:"CreatedAt" json:"CreatedAt"`
	UpdatedAt    time.Time              `bson:"UpdatedAt" json:"UpdatedAt"`
	Tags         map[string]interface{} `bson:"Tags" json:"Tags"`
}

// UserCount       int // probably don't need this
// UserIds         []primitive.ObjectID

type ChatMapToUserMarker struct {
	Marker string
}

// Implement the String method for the Person struct
func (p *ChatMapToUserMarker) String() string {
	return `ChatMapToUser`
}

// Custom JSON Marshaller for MongoChatConv
func (m *ChatMapToUserMarker) MarshalJSON() ([]byte, error) {
	return []byte(`"ChatMapToUser"`), nil
}

func (m *ChatMapToUserMarker) UnmarshalJSON(data []byte) error {
	// Assuming the JSON value is just a plain string for the Marker.
	// You need to unquote JSON strings, as the JSON representation of a string is quoted.
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	m.Marker = m.String() // You might want to convert `str` to the expected format or value
	return nil
}

//easyjson:json
type MongoChatMapToUser struct {
	Id          primitive.ObjectID     `bson:"_id" json:"_id"`
	Marker      ChatMapToUserMarker    `bson:"-" json:"Marker"`
	ChatId      primitive.ObjectID     `bson:"ChatId" json:"ChatId"` // ConvId
	UserId      primitive.ObjectID     `bson:"UserId" json:"UserId"`
	InviterId   primitive.ObjectID     `bson:"InviterId" json:"InviterId"`
	DateAdded   time.Time              `bson:"DateAdded" json:"DateAdded"`
	DateRemoved time.Time              `bson:"DateRemoved" json:"DateRemoved"`
	IsRemoved   bool                   `bson:"IsRemoved" json:"IsRemoved"`
	IsAdmin     bool                   `bson:"IsAdmin" json:"IsAdmin"`
	SoftDelete  bool                   `bson:"SoftDelete" json:"SoftDelete"`
	CreatedBy   primitive.ObjectID     `bson:"CreatedBy" json:"CreatedBy"`
	UpdatedBy   primitive.ObjectID     `bson:"UpdatedBy" json:"UpdatedBy"`
	CreatedAt   time.Time              `bson:"CreatedAt" json:"CreatedAt"`
	UpdatedAt   time.Time              `bson:"UpdatedAt" json:"UpdatedAt"`
	Tags        map[string]interface{} `bson:"Tags" json:"Tags"`
}

type ChatConvEventsMarker struct {
	Marker string
}

// Implement the String method for the Person struct
func (p *ChatConvEventsMarker) String() string {
	return `ChatConvEvent`
}

// Custom JSON Marshaller for MongoChatConv
func (m *ChatConvEventsMarker) MarshalJSON() ([]byte, error) {
	return []byte(`"ChatConvEvent"`), nil
}

func (m *ChatConvEventsMarker) UnmarshalJSON(data []byte) error {
	// Assuming the JSON value is just a plain string for the Marker.
	// You need to unquote JSON strings, as the JSON representation of a string is quoted.
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	m.Marker = m.String() // You might want to convert `str` to the expected format or value
	return nil
}

//easyjson:json
type MongoChatConvEvents struct {
	Id         primitive.ObjectID     `bson:"_id" json:"_id"`
	Marker     ChatConvEventsMarker   `bson:"-" json:"Marker"`
	ChatId     primitive.ObjectID     `bson:"ChatId" json:"ChatId"`
	Who        primitive.ObjectID     `bson:"UserId" json:"UserId"`
	DidWhat    []string               `bson:"DidWhat" json:"DidWhat"`
	ToWhom     primitive.ObjectID     `bson:"ToWhom" json:"ToWhom"`
	SoftDelete bool                   `bson:"SoftDelete" json:"SoftDelete"`
	CreatedBy  primitive.ObjectID     `bson:"CreatedBy" json:"CreatedBy"`
	UpdatedBy  primitive.ObjectID     `bson:"UpdatedBy" json:"UpdatedBy"`
	CreatedAt  time.Time              `bson:"CreatedAt" json:"CreatedAt"`
	UpdatedAt  time.Time              `bson:"UpdatedAt" json:"UpdatedAt"`
	Tags       map[string]interface{} `bson:"Tags" json:"Tags"`
}

func (m MongoChatMapToUser) printId() {
	m.printId()
}
