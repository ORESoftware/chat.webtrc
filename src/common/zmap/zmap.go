// source file path: ./src/common/zmap/zmap.go
package zmap

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	vbu "github.com/oresoftware/chat.webtrc/src/common/v-utils"
	vbl "github.com/oresoftware/chat.webtrc/src/common/vibelog"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

type ZMap struct {
	Mip *map[string]interface{}
}

func NewZMap(m interface{}) ZMap {
	v, ok := m.(*Map)
	if !ok {
		v, ok := m.(Map)
		if !ok {
			panic(vbu.ErrorFromArgs("b6b5fc5a-868e-441e-a0fc-5fac15f2a838:", "could not convert type to Map or *Map"))
		}
		return ZMap{&v}
	}
	return ZMap{v}
}

type Map = map[string]interface{}

func (z *ZMap) UnmarshalStringToMap(v string) (map[string]interface{}, error) {
	var m map[string]interface{}
	err := json.Unmarshal([]byte(v), &m)
	return m, err
}

func (z *ZMap) UnmarshalBytesToMap(v []byte) (map[string]interface{}, error) {
	var m map[string]interface{}
	err := json.Unmarshal(v, &m)
	return m, err
}

func (z *ZMap) Ls(s ...string) []string {
	var ret []string
	for _, v := range s {
		ret = append(ret, v)
	}
	return ret
}

func (z *ZMap) MustGetUUID(n string, f func(v uuid.UUID) error) error {
	if v, ok := (*z.Mip)[n]; ok {
		z, ok := v.(uuid.UUID)
		if ok {
			return f(z)
		}
		return vbu.ErrorFromArgs("ffb8d9e3-da00-4aaa-af0f-ad7f00b48da9", "was not a uuid")
	}
	return vbu.ErrorFromArgs("12bddfd8-1788-4dd8-aa84-2b176c7d9e59", "no key in map")
}

func (z *ZMap) GetUUID(n string, f func(v uuid.UUID) error) error {
	if v, ok := (*z.Mip)[n]; ok {
		z, ok := v.(uuid.UUID)
		if ok {
			return f(z)
		}
		return nil
	}
	return nil
}

func (z *ZMap) MustGetInt(n string, f func(v int) error) error {
	if v, ok := (*z.Mip)[n]; ok {
		z, ok := v.(int)
		if ok {
			return f(z)
		}
		return vbu.ErrorFromArgs("21ca86de-4b7f-451c-8f38-c39c4cc9cda7", "was not a string or length was less than 1")
	}
	return vbu.ErrorFromArgs("df25879e-e93f-4f46-8dad-0f1ee9feaeed", "no key in map")
}

func (z *ZMap) GetInt(n string, f func(v int) error) error {
	if v, ok := (*z.Mip)[n]; ok {
		z, ok := v.(int)
		if ok {
			return f(z)
		}
		return nil
	}
	return nil
}

func (z *ZMap) MustGetInt64(n string, f func(v int64) error) error {
	if v, ok := (*z.Mip)[n]; ok {
		z, ok := v.(int64)
		if ok {
			return f(z)
		}
		return vbu.ErrorFromArgs("5536e6ca-ad03-43ae-999c-ede1e4a93f55", "was not a string or length was less than 1")
	}
	return vbu.ErrorFromArgs("08173b1a-f9e1-474e-8d40-1bdeb0d3c87e", "no key in map")
}

func (z *ZMap) GetInt64(n string, f func(v int64) error) error {
	if v, ok := (*z.Mip)[n]; ok {
		z, ok := v.(int64)
		if ok {
			return f(z)
		}
		return nil
	}
	return nil
}

func (z *ZMap) MustGetBool(n string, f func(v bool) error) error {
	if v, ok := (*z.Mip)[n]; ok {
		if z, ok := v.(bool); ok {
			return f(z)
		} else {
			return vbu.ErrorFromArgs(
				"90ea1e31-714e-4537-85e4-724bb2dc072d",
				fmt.Sprintf("could not be cast to bool: %v", v),
			)
		}
	}
	return vbu.ErrorF(
		"fbb7b491-a803-4454-ba69-647bca68f03a",
		"the key '%s' was not in map",
		n,
	)
}

func (z *ZMap) GetBool(n string, f func(v bool) error) error {
	if v, ok := (*z.Mip)[n]; ok {
		if z, ok := v.(bool); ok {
			return f(z)
		} else {
			return vbu.ErrorFromArgs(
				"42d83d16-5f20-4aaf-9c46-8ca1d7ac0fcb",
				fmt.Sprintf("could not be cast to bool: %v", v),
			)
		}
	}
	return nil
}

func (z *ZMap) GetString(n string, f func(v string) error) error {
	if v, ok := (*z.Mip)[n]; ok {
		if z, ok := v.(string); ok {
			return f(z)
		} else {
			return vbu.ErrorFromArgs(
				"3dab16e3-2b0b-4b65-9781-9862801a1855",
				fmt.Sprintf("could not be cast to string: %v", v),
			)
		}
	}
	return nil
}

func (z *ZMap) MustGetString(n string, f func(v string) error) error {
	if v, ok := (*z.Mip)[n]; ok {
		z, ok := v.(string)
		if ok && len(z) > 0 {
			return f(z)
		}
		return vbu.ErrorF(
			"d470e708-3a8a-4c4f-8868-ce4c023501bd",
			"the key '%s' was not a string or length was less than 1",
			n,
		)
	}
	return errors.New(fmt.Sprintf("the key '%s' was not in map", n))
}

func (z *ZMap) MustGetMongoObjectId(n string, f func(v *primitive.ObjectID) error) error {
	if v, ok := (*z.Mip)[n]; ok {

		if z, ok := v.(primitive.ObjectID); ok && !z.IsZero() {
			return f(&z)
		}

		if d, ok := v.(*primitive.ObjectID); ok && !d.IsZero() {
			return f(d)
		}

		s, ok := v.(string)

		if ok {
			x, err := primitive.ObjectIDFromHex(s)
			if err != nil {
				return err
			}
			if x.IsZero() {
				return fmt.Errorf("object id was the zero object id.")
			}
			return f(&x)
		}

		return fmt.Errorf("the key '%s' was not a non-zero mongo objectid", n)
	}
	return fmt.Errorf("the key '%s' was not in map", n)
}

func (z *ZMap) GetMongoObjectIdFromString(n string, f func(v *primitive.ObjectID) error) error {
	if v, ok := (*z.Mip)[n]; ok {

		s, ok := v.(string)

		if !ok {
			return errors.New(fmt.Sprintf("this key '%s' was not a string type", n))
		}

		z, err := primitive.ObjectIDFromHex(s)

		if err != nil {
			return vbu.ErrorFromArgs(
				"857935cf-005a-4375-aaa3-e51681aa0e8f",
				fmt.Sprintf("the key '%s' has a problem: '%v'", s, err),
			)
		}

		if z.IsZero() {
			return errors.New(
				fmt.Sprintf("40f5c8f8-78a9-4397-9854-25c961887719: the key '%s' was not a non-zero mongo objectid", n),
			)
		}

		return f(&z)
	}
	return errors.New(fmt.Sprintf("the key '%s' was not in map", n))
}

func (z *ZMap) GetMongoObjectId(n string, f func(v *primitive.ObjectID) error) error {
	if v, ok := (*z.Mip)[n]; ok {
		z, ok := v.(primitive.ObjectID)
		if ok && !z.IsZero() {
			return f(&z)
		}
		d, ok := v.(*primitive.ObjectID)
		if ok && !d.IsZero() {
			return f(d)
		}
		return errors.New(
			fmt.Sprintf("4db24433-9856-4b3e-9299-cc0e417a3366: the key '%s' was not a non-zero mongo objectid", n),
		)
	}
	return nil
}

func (z *ZMap) GetTime(n string, f func(v time.Time) error) error {
	if v, ok := (*z.Mip)[n]; ok {
		switch v := v.(type) {
		case time.Time:
			return f(v)
		case *time.Time:
			return f(*v)
		case string:
			// Try to parse the string as a time.Time object
			parsedTime, err := time.Parse(time.RFC3339, v) // Assuming RFC3339 format; adjust as necessary
			if err != nil {
				return errors.New("value is a string but not a valid time format")
			}
			return f(parsedTime)
		default:
			return errors.New("value is neither time.Time nor string")
		}
	}
	return nil
}

func (z *ZMap) MustGetTime(n string, f func(v time.Time) error) error {
	if v, ok := (*z.Mip)[n]; ok {
		switch v := v.(type) {
		case time.Time:
			return f(v)
		case *time.Time:
			return f(*v)
		case string:
			// Try to parse the string as a time.Time object
			parsedTime, err := time.Parse(time.RFC3339, v) // Assuming RFC3339 format; adjust as necessary
			if err != nil {
				return errors.New("value is a string but not a valid time format")
			}
			return f(parsedTime)
		default:
			return errors.New("value is neither time.Time nor string")
		}
	}
	return errors.New("key not found")
}

func (z *ZMap) GetSlice(n string, f func(v []interface{}) error) error {
	if v, ok := (*z.Mip)[n]; ok {
		z, ok := v.([]interface{})
		if ok && len(z) > 0 {
			return f(z)
		}
		return errors.New(fmt.Sprintf("c8c51d48-6fb6-4121-99fb-855a661d9696: the key '%s' was not an array/slice", n))
	}
	return nil
}

func (z *ZMap) GetStringSlice(n string, f func(v *[]string) error) error {
	if v, ok := (*z.Mip)[n]; ok {
		z, ok := v.([]interface{})
		if !ok {
			return errors.New(fmt.Sprintf("5a0fd770-24aa-4316-a9b8-6d3190ebf7a8: the key '%s' was not an array/slice of", n))
		}
		var ret []string
		for _, x := range z {
			b, ok := x.(string)
			if !ok {
				return errors.New(fmt.Sprintf("46fbe119-dd67-448a-9d2c-466113efc230: the key '%s' was not an array/slice of", n))
			}
			ret = append(ret, b)
		}
		return f(&ret)
	}
	return nil
}

func (z *ZMap) MustGetBothStringAndObjectIdSlice(n string, f func(s *[]string, v *[]primitive.ObjectID) error) error {
	return z.MustGetStringSlice(n, func(ids *[]string) error {

		if ids == nil {
			return errors.New(fmt.Sprintf("9e3513cc-1e9a-4167-a849-6c5a3cab117d: the key '%s' was not in map", n))
		}

		strings := []string{}
		objectIds := []primitive.ObjectID{}

		for _, s := range *ids {

			var obj, err = primitive.ObjectIDFromHex(s)
			if err != nil {
				vbl.Stdout.Warn(vbl.Id("a5e87d29-2e7a-432f-b7ce-2028792b567d"), err)
				return err
			}
			if obj.IsZero() {
				var err = errors.New("f6af0f74-1368-48c4-881e-167b252e8a59: ObjectId is zeroed out")
				vbl.Stdout.Warn(vbl.Id("f17d8bff-bd6e-4229-97af-461b479dc016"), err)
				return err
			}

			strings = append(strings, obj.Hex())
			objectIds = append(objectIds, obj)
		}

		return f(&strings, &objectIds)
	})
}

func (z *ZMap) MustGetObjectIdSlice(n string, f func(v *[]primitive.ObjectID) error) error {
	return z.MustGetStringSlice(n, func(ids *[]string) error {

		if ids == nil {
			return errors.New(fmt.Sprintf("7007d72e-9740-4c54-b654-1448dad6e463: the key '%s' was not in map", n))
		}

		objectIds := []primitive.ObjectID{}

		for _, s := range *ids {

			var obj, err = primitive.ObjectIDFromHex(s)
			if err != nil {
				vbl.Stdout.Warn(vbl.Id("3d4af5f1-4f28-4fa1-804c-b79bb04f4094"), err)
				return err
			}
			if obj.IsZero() {
				var err = errors.New("e0d363c9-f10a-4443-b543-929724b14e88: ObjectId is zeroed out")
				vbl.Stdout.Warn(vbl.Id("1e7265e9-8537-44c4-bb24-1352b0587069"), err)
				return err
			}

			objectIds = append(objectIds, obj)
		}

		return f(&objectIds)
	})
}

func (z *ZMap) MustGetStringSlice(n string, f func(v *[]string) error) error {
	return z.GetStringSlice(n, func(v *[]string) error {
		if v == nil {
			return errors.New(fmt.Sprintf("b66bc61a-d67e-435c-8ddd-4ffe7ae63637: the key '%s' was not in map", n))
		} else {
			return f(v)
		}
	})
}

func (z *ZMap) MustGetSliceOf(n string, f func(v []interface{}) error) error {
	if v, ok := (*z.Mip)[n]; ok {
		z, ok := v.([]interface{})
		if ok {
			return f(z)
		}
		return errors.New(fmt.Sprintf("70d35823-b085-46e2-8370-4ce06d4ec2fb: the key '%s' was not an array/slice", n))
	}
	return errors.New(fmt.Sprintf("faf6d8b9-8bf4-4ac1-8b2d-2fd95f547756: the key '%s' was not in map", n))
}

func (z *ZMap) MustGetSlice(n string, f func(v []interface{}) error) error {
	if v, ok := (*z.Mip)[n]; ok {
		z, ok := v.([]interface{})
		if ok {
			return f(z)
		}
		return errors.New(fmt.Sprintf("dbb25701-2dee-40a6-93e9-11090e38cb46: the key '%s' was not an array/slice", n))
	}
	return errors.New(fmt.Sprintf("78f9d198-f4c9-4ab9-a788-aa46fd7e525f: the key '%s' was not in map", n))
}

func (z *ZMap) MustGetJSON(n string, f func(v []byte) error) error {
	if v, ok := (*z.Mip)[n]; ok {

		x, err := json.Marshal(v)

		if err != nil {
			return err
		}

		if ok && len(x) > 0 {
			return f([]byte(x))
		}

		return errors.New("was not a string or length was less than 1")
	}
	return errors.New("no key in map")
}

// func (z *ZMap) GetJSON(n string, f func(v null.JSON) error) error {
//	if v, ok := (*z.Mip)[n]; ok {
//
//		x, err := json.Marshal(v)
//
//		if err != nil {
//			return err
//		}
//
//		if ok {
//			return f(null.JSON{
//				JSON:  []byte(x),
//				Valid: true,
//			})
//		}
//		return nil
//	}
//	return nil
// }

// func GetJSON(m Map, n string, f func(v null.JSON) error) error {
//	if v, ok := m[n]; ok {
//
//		x, err := json.Marshal(v)
//
//		if err != nil {
//			return err
//		}
//
//		if ok {
//			return f(null.JSON{
//				JSON:  []byte(x),
//				Valid: true,
//			})
//		}
//		return nil
//	}
//	return nil
// }

//
// var ZMapper = make(map[string](func(Map, string, func(interface{})(error))error))
//
// func init(){
//  ZMapper["string"] = GetJSON
// }
