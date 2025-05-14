package main

import (
	"fmt"
	"github.com/pion/webrtc/v3" // Import the package
)

func main() {
	// Now you can use types/functions from the webrtc package
	fmt.Println("Hello, project using WebRTC!")
	// Example using a type from the package (won't compile without proper usage,
	// but shows the import works)
	var api *webrtc.API
	_ = api // Use the variable to avoid unused error
}
