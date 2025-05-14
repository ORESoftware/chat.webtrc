// source file path: ./src/common/push-notif/ios/push-notif-ios.go
package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

// APNSPayload represents the payload to be sent to APNS
type APNSPayload struct {
	Aps struct {
		Alert string `json:"alert"`
	} `json:"aps"`
}

func Push() {
	// Load your APNS certificate
	cert, err := tls.LoadX509KeyPair("path/to/certificate.pem", "path/to/privatekey.pem")
	if err != nil {
		log.Fatal("Error loading certificate:", err)
	}

	// Setup HTTPS client
	caCertPool := x509.NewCertPool()
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates: []tls.Certificate{cert},
				RootCAs:      caCertPool,
			},
		},
	}

	// Create payload
	payload := APNSPayload{}
	payload.Aps.Alert = "Hello, APNS!"

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Fatal("Error marshalling payload:", err)
	}

	// Create request
	req, err := http.NewRequest("POST", "https://api.development.push.apple.com/3/device/<device_token>", bytes.NewReader(payloadBytes))
	if err != nil {
		log.Fatal("Error creating request:", err)
	}
	req.Header.Set("apns-topic", "<your_app_bundle_id>")

	// Send request
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
