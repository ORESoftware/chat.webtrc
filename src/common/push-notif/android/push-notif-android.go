// source file path: ./src/common/push-notif/android/push-notif-android.go
package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

// FCMPayload represents the payload to be sent to FCM
type FCMPayload struct {
	To   string `json:"to"`
	Data struct {
		Message string `json:"message"`
	} `json:"data"`
}

func Push() {
	// Your FCM server key
	serverKey := "YOUR_SERVER_KEY"

	// Create payload
	payload := FCMPayload{
		To: "/topics/YOUR_TOPIC_OR_DEVICE_TOKEN",
	}
	payload.Data.Message = "Hello, FCM!"

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Fatal("Error marshalling payload:", err)
	}

	// Create request
	req, err := http.NewRequest("POST", "https://fcm.googleapis.com/fcm/send", bytes.NewReader(payloadBytes))
	if err != nil {
		log.Fatal("Error creating request:", err)
	}
	req.Header.Set("Authorization", "key="+serverKey)
	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Error sending request:", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error reading response:", err)
	}

	log.Println("Response status:", resp.Status)
	log.Println("Response body:", string(body))
}
