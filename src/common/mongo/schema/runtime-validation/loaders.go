// source file path: ./src/common/mongo/schema/runtime-validation/loaders.go
package v_runtime_validation

import (
	"fmt"
	"github.com/xeipuuv/gojsonschema"
	"io"
	"log"
	"os"
)

var pwd = os.Getenv("PWD")

func readFile(fp string) string {

	var filePath = fmt.Sprintf("%s/%s/%s", pwd, "src/common/mongo/schema", fp)
	// Replace 'path/to/your/file.txt' with the actual file path

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("failed to open file: %s", err)
	}
	defer file.Close()

	// Read the file into a byte slice
	data, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("failed to read file: %s", err)
	}

	// Convert the byte slice to a string
	return string(data)

}

var ChatUserSchema = gojsonschema.NewStringLoader(
	readFile("values/MongoChatUser.schema.json"),
)

var ChatConvSchema = gojsonschema.NewStringLoader(
	readFile("values/MongoChatConv.schema.json"),
)

var MessageAckSchema = gojsonschema.NewStringLoader(
	readFile("values/MongoMessageAck.schema.json"),
)

var ChatMessageSchema = gojsonschema.NewStringLoader(
	readFile("values/MongoChatMessage.schema.json"),
)

var ChatUserDeviceSchema = gojsonschema.NewStringLoader(
	readFile("values/MongoChatUserDevice.schema.json"),
)

var ChatMapToUserSchema = gojsonschema.NewStringLoader(
	readFile("values/MongoChatMapToUser.schema.json"),
)
