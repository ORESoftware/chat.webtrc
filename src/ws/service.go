package virl_ws

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	vapm "github.com/oresoftware/chat.webtrc/src/common/apm"
	"github.com/oresoftware/chat.webtrc/src/common/gor"
	vhp "github.com/oresoftware/chat.webtrc/src/common/handle-panic"
	vjwt "github.com/oresoftware/chat.webtrc/src/common/jwt" // Still needed for auth check logic
	virl_mongo "github.com/oresoftware/chat.webtrc/src/common/mongo"
	mngo_types "github.com/oresoftware/chat.webtrc/src/common/mongo/types" // Still needed for Mongo types
	vctx "github.com/oresoftware/chat.webtrc/src/common/mongo/vctx" // Still needed for Mongo context
	vbu "github.com/oresoftware/chat.webtrc/src/common/v-utils" // Still needed for utils
	virl_err "github.com/oresoftware/chat.webtrc/src/common/verrors" // Still needed for errors
	vbl "github.com/oresoftware/chat.webtrc/src/common/vibelog" // Still needed
	virl_conf "github.com/oresoftware/chat.webtrc/src/config" // Still needed
	vuc "github.com/oresoftware/chat.webtrc/src/ws/uc" // Still needed

	"github.com/gorilla/websocket" // Still needed for websockets
	"github.com/mailru/easyjson" // Still needed for easyjson unmarshalling
	"github.com/pion/webrtc/v3" // Add Pion import
)

// Removed broker-specific types (KafkaMap, RedisMap, RabbitMap, ChanCount, WriterCounter)

// Service holds the server's state and handles WebSocket connections and WebRTC signaling.
type Service struct {
	config *virl_conf.ConfigVars // Application configuration

	// Maps to keep track of active WebSocket connections, keyed by user ID and conversation ID
	// A user might have multiple connections, and a conversation involves multiple users/connections.
	userIdToConns map[string]map[*vuc.UserCtx]*vuc.UserCtx
	convIdToConns map[string]map[*vuc.UserCtx]*vuc.UserCtx // Map conversation ID to set of UserCtx pointers
	mu            sync.Mutex                              // Mutex for protecting userIdToConns and convIdToConns maps

	M *virl_mongo.M // MongoDB client wrapper

	webrtcConfig webrtc.Configuration // WebRTC configuration (STUN/TURN servers)
	// webrtcPCsMtx sync.Mutex // Moved to UserCtx as PCs are per connection

	killOnce *sync.Once // Used for killOldSubs if kept/reimplemented for WS connections

	// Removed broker-specific connection pools and configs
	// RMQConnectionPool, RedisConnPool, KafkaProducerConnPool, KafkaConsumerConnPool, KafkaProducerConf, KafkaConsumerConf removed
}

// NewService creates a new instance of the Service.
func NewService(cfg *virl_conf.ConfigVars) *Service {
	var s = Service{
		config:        cfg,
		convIdToConns: make(map[string]map[*vuc.UserCtx]*vuc.UserCtx), // Use make for maps
		userIdToConns: make(map[string]map[*vuc.UserCtx]*vuc.UserCtx), // Use make for maps
		M:             nil, // Set later in main
		killOnce:      &sync.Once{}, // Needed for killOldSubs if kept
		webrtcConfig: webrtc.Configuration{ // Initialize WebRTC configuration based on config
			ICEServers: cfg.GetWebRTCIceServers(), // Use the parsed ICE servers from config
		},
		mu:            sync.Mutex{}, // Initialize the mutex
	}

	// Start goroutine for logging memory stats (simplified)
	go func() {
		for {
			time.Sleep(50 * time.Second)
			s.logMemoryStats()
		}
	}()

	// TODO fix this - killOldSubs needs to be adapted or removed
	s.killOldSubs() // This function needs review after removing brokers

	// Removed Redis and Kafka connection pool setup
	// RMQConnectionPool is also not initialized here, assumed to be passed from main if needed elsewhere (it isn't used internally now)

	return &s
}

// Helper function to count total connections in a map of maps
func countTotalConns(m map[string]map[*vuc.UserCtx]*vuc.UserCtx) int {
	count := 0
	// No need for mutex here as this is called within the Service mutex
	for _, innerMap := range m {
		count += len(innerMap)
	}
	return count
}

// logMemoryStats logs statistics about active connections.
func (s *Service) logMemoryStats() {
	// Removed broker map logging

	go func() {
		defer func() {
			if r := vhp.HandlePanic("cd172d55afae"); r != nil {
				vapm.SendTrace("ec3113ca-978e-4fd9-8494-41e51ca7da01", fmt.Sprintf("%v", r))
				vbl.Stdout.Error(vbl.Id("vid/0bc5a0ea277d"), fmt.Sprintf("%v", r))
			}
		}()

		s.mu.Lock() // Protect the connection maps during iteration/counting
		vbl.Stdout.Warn(
			vbl.Id("vid/2b7214e91c69"),
			fmt.Sprintf("convIdToConns: keys=%d, total_conns=%d", len(s.convIdToConns), countTotalConns(s.convIdToConns)),
			fmt.Sprintf("userIdToConns: keys=%d, total_conns=%d", len(s.userIdToConns), countTotalConns(s.userIdToConns)),
		)
		s.mu.Unlock()
	}()
}

// handleNewMongoUser handles logic when a new chat user is created (not used by WS directly now).
func (s *Service) handleNewMongoUser(m mngo_types.MongoChatUser) error {
	return nil // Placeholder
}

// handleNewMongoConversation handles logic when a new chat conversation is created (not used by WS directly now).
func (s *Service) handleNewMongoConversation(m mngo_types.MongoChatConv) error {
	// Should be an HTTP request instead
	return nil // Placeholder
}

// handleChatMessageAck handles saving message acknowledgements.
func (s *Service) handleChatMessageAck(
	uc *vuc.UserCtx,
	usersToNotify []primitive.ObjectID, // Users who should be notified about this ack
	originalAuthor *primitive.ObjectID, // Author of the message being acknowledged
	originalCreatedAt *time.Time, // Timestamp of the message being acknowledged
	m *mngo_types.MongoMessageAck, // The acknowledgement document
) error {

	vbl.Stdout.Warn("dcc6d9f7-276d-40e6-a1fc-5325f63cb9c6", "usersToNotify:", usersToNotify, "chat Ack:", m)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*7)
	defer cancel() // Ensure context is cancelled
	coll := s.M.Col.ChatConvMessageAck

	if _, err := s.M.DoInsertOne(vctx.NewMgoCtx(&ctx), coll, m); err != nil {
		vbl.Stdout.Warn(vbl.Id("vid/b167a09fb1de"), fmt.Sprintf("Error inserting message ack: %v", err))
	}

	// TODO: Notify relevant users that the message has been acknowledged.
	// This would involve finding the target user(s) and sending a specific WS message.
	// For now, this notification logic (sending to original author, usersToNotify) is commented out/removed.

	if originalAuthor == nil || originalAuthor.IsZero() {
		vbl.Stdout.Warn(vbl.Id("vid/4ccda144ba1f"), "the object id for original author is nil or zero id")
		return nil
	}

	// Removed notification logic (sendChatMessageOut calls)

	return nil
}

// handleUserLocation handles updating user location (placeholder).
func (s *Service) handleUserLocation(clientId string, m *mngo_types.MongoUserLocation) error {
	// TODO: Implement logic to save location and potentially broadcast to interested parties
	return nil // Placeholder
}

// lockApp is a placeholder for locking the server (not fully implemented).
func (s *Service) lockApp() error {
	// TODO: Implement a mechanism to pause incoming connections or requests
	s.mu.Lock() // This only locks the connection maps, not the entire app server
	return nil
}

// countTotalConns is a helper (defined above)

// Removed calculateMapSize and calculateGenericMapSize

// WSIncomingMsg represents the standard structure of messages received over WebSocket.
// easyjson:json
type WSIncomingMsg struct {
	Meta interface{} `json:"Meta"`
	List []map[string]interface{} `json:"List"` // Each item in the list is a message/event
}


// Removed copyMap

// handleWebRTCMessage processes incoming WebRTC signaling messages.
// These messages are received over the existing WebSocket connection.
func (s *Service) handleWebRTCMessage(uc *vuc.UserCtx, vibeData interface{}, metaD map[string]interface{}, callTrace []string) error {
	var cs = callTrace // Use the provided callTrace

	// Struct to unmarshal the content of the "@vibe-data" field for WebRTC signaling messages.
	// This struct is also defined in uc/user-ctx.go, ensure consistency.
	// easyjson:json
	type WebRTCSignalingPayloadData struct {
		Type    string `json:"type"`    // "offer", "answer", "ice", "close"
		CallId  string `json:"callId"`  // Call identifier (needed to find the right PeerConnection)
		Payload json.RawMessage `json:"payload"` // The actual SDP or ICE candidate payload (keep as RawMessage for flexibility)
	}

	var signalingPayload WebRTCSignalingPayloadData

	// Unmarshal the interface{} payload. We need to marshal it to bytes first.
	dataBytes, err := json.Marshal(vibeData)
	if err != nil {
		vbl.Stdout.Error(vbl.Id("vid/webrtc_marshal_err"), fmt.Sprintf("Error marshaling vibeData for signaling: %v, data: %+v", err, vibeData))
		uc.WriteJSONSafely(cs, s.VibeChatErr("webrtc_marshal_err", fmt.Errorf("malformed webrtc signaling message")))
		return err
	}

	if err := easyjson.Unmarshal(dataBytes, &signalingPayload); err != nil {
		vbl.Stdout.Error(vbl.Id("vid/webrtc_unmarshal_err"), fmt.Sprintf("Error unmarshaling signaling payload: %v, data: %s", err, string(dataBytes)))
		uc.WriteJSONSafely(cs, s.VibeChatErr("webrtc_unmarshal_err", fmt.Errorf("malformed webrtc signaling message")))
		return err
	}

	callId := signalingPayload.CallId
	if callId == "" {
		vbl.Stdout.Warn(vbl.Id("vid/webrtc_missing_callid"), "WebRTC signaling message missing callId")
		uc.WriteJSONSafely(cs, s.VibeChatErr("webrtc_missing_callid", fmt.Errorf("webrtc signaling message missing callId")))
		return fmt.Errorf("missing callId in webrtc signaling message")
	}

	// Find the PeerConnection for this callId on this user's context.
	// The PeerConnection *must* exist for "answer", "ice", "data", "close" types.
	// For "offer", it will be created.
	uc.webrtcPCsMtx.Lock()
	pc, pcExists := uc.webrtcPCs[callId]
	uc.webrtcPCsMtx.Unlock()


	switch signalingPayload.Type {
	case "offer":
		vbl.Stdout.Info(vbl.Id("vid/webrtc_handle_offer"), fmt.Sprintf("Handling WebRTC offer for call %s (user %s)", callId, uc.UserId))

		// Create the PeerConnection if it doesn't exist. This peer is the answerer.
		if !pcExists {
			var createErr error
			// isOfferer is false because this peer received the offer
			pc, createErr = uc.GetOrCreatePeerConnection(cs, callId, s.webrtcConfig, false)
			if createErr != nil {
				vbl.Stdout.Error(vbl.Id("vid/webrtc_create_pc_on_offer_err"), fmt.Sprintf("Error creating PC on offer for call %s (user %s): %v", callId, uc.UserId, createErr))
				uc.WriteJSONSafely(cs, s.VibeChatErr("webrtc_create_pc_on_offer_err", fmt.Errorf("server failed to create webrtc connection")))
				return createErr // Return the error to stop processing this message
			}
		} else {
			vbl.Stdout.Warn(vbl.Id("vid/webrtc_offer_pc_exists"), fmt.Sprintf("Received offer for call %s (user %s), but PC already exists. Reusing existing PC.", callId, uc.UserId))
			// Decide how to handle this: maybe reset the PC? For now, just reuse and warn.
			// A robust implementation might require a more specific signaling flow (e.g., "start-call" before "offer")
		}


		// Unmarshal the SDP offer payload
		// easyjson:json
		type OfferPayload struct {
			SDP string `json:"sdp"` // The SDP string
		}
		var offerPayload OfferPayload
		if err := easyjson.Unmarshal(signalingPayload.Payload, &offerPayload); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/webrtc_unmarshal_offer_err"), fmt.Sprintf("Error unmarshaling offer payload for call %s (user %s): %v, payload: %s", callId, uc.UserId, err, string(signalingPayload.Payload)))
			uc.WriteJSONSafely(cs, s.VibeChatErr("webrtc_unmarshal_offer_err", fmt.Errorf("malformed offer payload")))
			return err
		}

		sdp := webrtc.SessionDescription{
			Type: webrtc.SDPTypeOffer,
			SDP:  offerPayload.SDP,
		}

		// Set the remote description (the offer)
		if err := pc.SetRemoteDescription(sdp); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/webrtc_set_remote_offer_err"), fmt.Sprintf("Error setting remote offer for call %s (user %s): %v", callId, uc.UserId, err))
			uc.WriteJSONSafely(cs, s.VibeChatErr("webrtc_set_remote_offer_err", fmt.Errorf("server failed to set remote offer")))
			return err
		}

		// Create an answer
		answer, err := pc.CreateAnswer(nil)
		if err != nil {
			vbl.Stdout.Error(vbl.Id("vid/webrtc_create_answer_err"), fmt.Sprintf("Error creating answer for call %s (user %s): %v", callId, uc.UserId, err))
			uc.WriteJSONSafely(cs, s.VibeChatErr("webrtc_create_answer_err", fmt.Errorf("server failed to create answer")))
			return err
		}

		// Set the local description (the answer)
		if err := pc.SetLocalDescription(answer); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/webrtc_set_local_answer_err"), fmt.Sprintf("Error setting local answer for call %s (user %s): %v", callId, uc.UserId, err))
			uc.WriteJSONSafely(cs, s.VibeChatErr("webrtc_set_local_answer_err", fmt.Errorf("server failed to set local answer")))
			return err
		}

		// Send the answer back to the offerer client over WebSocket signaling
		// The payload must include the callId.
		answerPayload := map[string]interface{}{
			"callId": callId,
			"sdp":    answer.SDP,
			"type":   answer.Type.String(), // "answer"
			"originatorUserId": uc.UserId, // Add sender info
		}
		if sendErr := uc.SendWebRTCMessage(cs, "answer", answerPayload); sendErr != nil {
			vbl.Stdout.Error(vbl.Id("vid/webrtc_send_answer_err"), fmt.Sprintf("Error sending answer for call %s (user %s): %v", callId, uc.UserId, sendErr))
		}


	case "answer":
		vbl.Stdout.Info(vbl.Id("vid/webrtc_handle_answer"), fmt.Sprintf("Handling WebRTC answer for call %s (user %s)", callId, uc.UserId))

		// PeerConnection *must* exist if we're handling an answer.
		if !pcExists || pc == nil {
			vbl.Stdout.Warn(vbl.Id("vid/webrtc_answer_pc_not_found"), fmt.Sprintf("Received answer for callId %s (user %s), but PC not found.", callId, uc.UserId))
			uc.WriteJSONSafely(cs, s.VibeChatErr("webrtc_answer_pc_not_found", fmt.Errorf("received answer for unknown call %s", callId)))
			return fmt.Errorf("peer connection not found for callId %s", callId)
		}


		// Unmarshal the SDP answer payload
		// easyjson:json
		type AnswerPayload struct {
			SDP string `json:"sdp"` // The SDP string
		}
		var answerPayload AnswerPayload
		if err := easyjson.Unmarshal(signalingPayload.Payload, &answerPayload); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/webrtc_unmarshal_answer_err"), fmt.Sprintf("Error unmarshaling answer payload for call %s (user %s): %v, payload: %s", callId, uc.UserId, err, string(signalingPayload.Payload)))
			uc.WriteJSONSafely(cs, s.VibeChatErr("webrtc_unmarshal_answer_err", fmt.Errorf("malformed answer payload")))
			return err
		}

		sdp := webrtc.SessionDescription{
			Type: webrtc.SDPTypeAnswer, // Explicitly set type as Answer
			SDP:  answerPayload.SDP,
		}

		// Set the remote description (the answer)
		if err := pc.SetRemoteDescription(sdp); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/webrtc_set_remote_answer_err"), fmt.Sprintf("Error setting remote answer for call %s (user %s): %v", callId, uc.UserId, err))
			uc.WriteJSONSafely(cs, s.VibeChatErr("webrtc_set_remote_answer_err", fmt.Errorf("server failed to set remote answer")))
			return err
		}


	case "ice":
		vbl.Stdout.Info(vbl.Id("vid/webrtc_handle_ice"), fmt.Sprintf("Handling WebRTC ICE candidate for call %s (user %s)", callId, uc.UserId))

		// PeerConnection *must* exist if we're handling an ICE candidate.
		if !pcExists || pc == nil {
			vbl.Stdout.Warn(vbl.Id("vid/webrtc_ice_pc_not_found"), fmt.Sprintf("Received ICE candidate for callId %s (user %s), but PC not found.", callId, uc.UserId))
			uc.WriteJSONSafely(cs, s.VibeChatErr("webrtc_ice_pc_not_found", fmt.Errorf("received ice candidate for unknown call %s", callId)))
			return fmt.Errorf("peer connection not found for callId %s", callId)
		}


		// Unmarshal the ICE candidate payload
		// easyjson:json
		type IcePayload struct {
			Candidate     string  `json:"candidate"` // The ICE candidate string (SDP attribute)
			SDPMid        *string `json:"sdpMid"` // The media stream identification
			SDPMLineIndex *uint16 `json:"sdpMLineIndex"` // The index of the m-line in the SDP
		}
		var icePayload IcePayload
		if err := easyjson.Unmarshal(signalingPayload.Payload, &icePayload); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/webrtc_unmarshal_ice_err"), fmt.Sprintf("Error unmarshaling ice payload for call %s (user %s): %v, payload: %s", callId, uc.UserId, err, string(signalingPayload.Payload)))
			uc.WriteJSONSafely(cs, s.VibeChatErr("webrtc_unmarshal_ice_err", fmt.Errorf("malformed ice payload")))
			return err
		}

		candidate := webrtc.ICECandidateInit{
			Candidate:     icePayload.Candidate,
			SDPMid:        icePayload.SDPMid,
			SDPMLineIndex: icePayload.SDPMLineIndex,
		}

		// Add the ICE candidate to the PeerConnection
		if err := pc.AddICECandidate(candidate); err != nil {
			// This can sometimes fail if the candidate is received before the remote description is set.
			// Pion usually handles this gracefully internally by queueing candidates.
			// Log a warning but don't necessarily return an error that stops the WS message processing loop.
			vbl.Stdout.Warn(vbl.Id("vid/webrtc_add_ice_err"), fmt.Sprintf("Error adding ICE candidate for call %s (user %s): %v", callId, uc.UserId, err))
			// Decide if you want to signal this failure back to the client.
			// uc.WriteJSONSafely(cs, s.VibeChatErr("webrtc_add_ice_err", fmt.Errorf("server failed to add ice candidate")))
			// Do NOT return the error here to allow other messages to be processed.
		}


	case "close":
		vbl.Stdout.Info(vbl.Id("vid/webrtc_handle_close"), fmt.Sprintf("Handling WebRTC close signal for call %s (user %s)", callId, uc.UserId))
		uc.RemovePeerConnection(callId) // Clean up PC


	case "data":
		// This case handles messages received over the *WebSocket*
		// that represent data intended to be relayed or processed by the server
		// in the context of a WebRTC call. It's *not* for data received *over* the
		// WebRTC DataChannel (that's handled by the DataChannel's OnMessage handler).

		vbl.Stdout.Info(vbl.Id("vid/webrtc_handle_data_ws"), fmt.Sprintf("Handling WebRTC data message received over WS for call %s (user %s)", callId, uc.UserId))

		// The payload here is the actual DataChannel message content sent over WS
		// easyjson:json
		type DataPayload struct {
			Message string `json:"message"` // Example: plain text data
			// Add fields to identify sender/recipient if needed for relaying
			FromUserId string `json:"fromUserId"` // The user who sent this message over their WS
			// ToUserId string `json:"toUserId"` // For point-to-point relaying
		}
		var dataPayload DataPayload
		if err := easyjson.Unmarshal(signalingPayload.Payload, &dataPayload); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/webrtc_unmarshal_data_err"), fmt.Sprintf("Error unmarshaling data payload for call %s (user %s): %v, payload: %s", callId, uc.UserId, err, string(signalingPayload.Payload)))
			uc.WriteJSONSafely(cs, s.VibeChatErr("webrtc_unmarshal_data_err", fmt.Errorf("malformed data payload")))
			return err
		}

		vbl.Stdout.Debug(vbl.Id("vid/webrtc_data_ws_content"), fmt.Sprintf("Received DataChannel content over WS for call %s from user %s: %s", callId, dataPayload.FromUserId, dataPayload.Message))

		// TODO: Implement logic to relay this message to other participants in the call.
		// This would typically involve:
		// 1. Identifying other UserCtx objects associated with this callId.
		// 2. Finding the active DataChannel(s) on the PeerConnections for those users.
		// 3. Sending the data message over those DataChannels.
		// This requires a service-level mapping of callIds to UserCtx objects or conversation IDs,
		// and access to the DataChannel references stored in each UserCtx.
		// For now, we'll just log it and send a WS confirmation back.

		// Example: Send back a confirmation/status over WS to the sender
		ackPayload := map[string]interface{}{
			"callId": callId,
			"type": "data_ack", // A specific type for the ack message
			"status": "received_by_server",
			"originalMessage": dataPayload, // Include the original message for context
			"serverTimestamp": time.Now().UTC().String(),
		}
		if sendErr := uc.SendWebRTCMessage(cs, "data_ack", ackPayload); sendErr != nil {
			vbl.Stdout.Warn(vbl.Id("vid/webrtc_send_data_ack_err"), fmt.Sprintf("Error sending data ack for call %s (user %s): %v", callId, uc.UserId, sendErr))
		}


	default:
		vbl.Stdout.Warn(vbl.Id("vid/webrtc_unknown_type"), fmt.Sprintf("Received unknown WebRTC signaling type '%s' for call %s (user %s)", signalingPayload.Type, callId, uc.UserId))
		uc.WriteJSONSafely(cs, s.VibeChatErr("webrtc_unknown_type", fmt.Errorf("unknown webrtc signaling type: %s", signalingPayload.Type)))
	}

	return nil
}


// handleIncomingWSMessages reads incoming messages from the WebSocket and processes them.
func (s *Service) handleIncomingWSMessages(uc *vuc.UserCtx, z gor.Updater) {

	// This defer block handles cleanup when the goroutine exits (usually because the connection is closed)
	defer func() {
		// This cleanup runs AFTER the loop exits (either due to error or planned closure)
		vbl.Stdout.Debug(vbl.Id("vid/ws_handle_msg_defer"), fmt.Sprintf("handleIncomingWSMessages goroutine exiting for user %s, device %s.", uc.UserId, uc.DeviceId))

		s.logMemoryStats() // Log memory stats after a connection closes
		s.doAudit() // Perform audit

		// uc.SafeClose handles closing the WS connection and all associated PeerConnections
		// This is now the primary cleanup entry point for a single connection.
		// Errors from SafeClose are logged internally.
		uc.SafeClose(false) // Attempt graceful close, don't ignore errors during close attempts


		// Remove the client and its connection from the Service maps when the connection is closed.
		// This must be done after SafeClose to avoid race conditions with sending messages during cleanup.
		s.mu.Lock() // Protect Service maps
		defer s.mu.Unlock() // Ensure mutex is released

		// Remove from userIdToConns map
		if conns, ok := s.userIdToConns[uc.UserId]; ok && conns != nil {
			delete(conns, uc) // Delete the specific connection pointer
			if len(conns) == 0 {
				delete(s.userIdToConns, uc.UserId) // If no more connections for this user, remove the user entry
				vbl.Stdout.Info(vbl.Id("vid/ws_user_map_cleanup"), fmt.Sprintf("Removed user %s from userIdToConns map.", uc.UserId))
			} else {
				vbl.Stdout.Debug(vbl.Id("vid/ws_user_map_partial_cleanup"), fmt.Sprintf("Removed one connection for user %s. %d remaining.", uc.UserId, len(conns)))
			}
		}


		// Remove from convIdToConns map for all conversations this UC was in.
		// Iterate over a copy of the ConvIds slice because `uc.ConvIds` shouldn't change
		// during this cleanup, but the maps might.
		// A safer approach is to iterate over the actual map entries referencing this UC.
		// However, the Service map iteration should be done under its mutex.
		// Let's iterate the UC's ConvIds slice, and access the Service map under mutex.
		for _, convId := range uc.ConvIds {
			s.mu.Lock() // Re-lock for accessing convIdToConns (might be better to use a single lock for defer)
			if conns, ok := s.convIdToConns[convId]; ok && conns != nil {
				delete(conns, uc) // Delete the specific connection pointer
				if len(conns) == 0 {
					delete(s.convIdToConns, convId) // If no more connections for this conv, remove the conv entry
					vbl.Stdout.Info(vbl.Id("vid/ws_conv_map_cleanup"), fmt.Sprintf("Removed conversation %s from convIdToConns map.", convId))
				} else {
					vbl.Stdout.Debug(vbl.Id("vid/ws_conv_map_partial_cleanup"), fmt.Sprintf("Removed one connection for conv %s. %d remaining.", convId, len(conns)))
				}
			}
			s.mu.Unlock() // Unlock after accessing/modifying the inner map
		}
		// The single Service mutex defer at the start of the defer block is sufficient if all map operations are inside it.
		// Let's restructure the defer block slightly to use a single mutex.

	}() // End defer block


	// Restructured defer block to use a single mutex for map cleanup
	/*
	    defer func() {
	        vbl.Stdout.Debug(vbl.Id("vid/ws_handle_msg_defer"), fmt.Sprintf("handleIncomingWSMessages goroutine exiting for user %s, device %s.", uc.UserId, uc.DeviceId))

	        // uc.SafeClose handles closing the WS connection and all associated PeerConnections
	        // This is now the primary cleanup entry point for a single connection.
	        uc.SafeClose(false) // Attempt graceful close

	        s.mu.Lock() // Lock the Service maps for cleanup
	        defer s.mu.Unlock() // Ensure mutex is released

	        s.logMemoryStats() // Log memory stats after a connection closes
			s.doAudit() // Perform audit

	        // Remove from userIdToConns map
	        if conns, ok := s.userIdToConns[uc.UserId]; ok && conns != nil {
	            delete(conns, uc)
	            if len(conns) == 0 {
	                delete(s.userIdToConns, uc.UserId)
	                vbl.Stdout.Info(vbl.Id("vid/ws_user_map_cleanup"), fmt.Sprintf("Removed user %s from userIdToConns map.", uc.UserId))
	            }
	        }

	        // Remove from convIdToConns map
	        for _, convId := range uc.ConvIds {
	            if conns, ok := s.convIdToConns[convId]; ok && conns != nil {
	                delete(conns, uc)
	                if len(conns) == 0 {
	                    delete(s.convIdToConns, convId)
	                    vbl.Stdout.Info(vbl.Id("vid/ws_conv_map_cleanup"), fmt.Sprintf("Removed conversation %s from convIdToConns map.", convId))
	                }
	            }
	        }
	    }() // End restructured defer block
	*/


	var cs = vbu.GetFilteredStacktrace() // Get initial call stack

	// Buffered channel to signal that the read loop has started.
	// This helps ensure the "ListeningAck" is sent after the read loop is active.
	readStartSignal := make(chan struct{}, 1) // Use empty struct for signal

	// Goroutine to send the "ListeningAck" after the read loop is ready.
	gor.Gor(func(z gor.Updater) {
		z.UpdateLastActive(time.Now(), gor.LastActiveInfo{
			Info: fmt.Sprintf("Waiting to send ListeningAck for user %s, device %s", uc.UserId, uc.DeviceId),
		})

		// Wait for the signal from the read loop
		select {
		case <-readStartSignal:
			vbl.Stdout.Debug(vbl.Id("vid/ws_listening_ack_signal"), fmt.Sprintf("Received read start signal for user %s. Sending ListeningAck.", uc.UserId))
			// Send the ListeningAck message to the client over WS
			ackMsg := map[string]interface{}{"Marker": "ListeningAck", "Listening": true}
			// Wrap in the standard outgoing payload structure
			outgoingPayload := WebsocketOutgoingPayload{
				Meta: WSOutgoingMeta{FromSource: "Server", TimeCreated: time.Now().UTC().String()},
				Messages: []interface{}{ackMsg},
			}
			if err := uc.WriteJSONSafely(cs, outgoingPayload); err != nil {
				vbl.Stdout.Warn(vbl.Id("vid/ws_listening_ack_write_err"), fmt.Sprintf("Error sending ListeningAck to user %s: %v", uc.UserId, err))
			} else {
				vbl.Stdout.Info(vbl.Id("vid/ws_listening_ack_sent"), fmt.Sprintf("Sent ListeningAck to user %s.", uc.UserId))
			}
		case <-z.GetTimer().C: // Timer from gor.Gor for detecting stalled goroutines
			vbl.Stdout.Warn(vbl.Id("vid/ws_listening_ack_timeout"), fmt.Sprintf("ListeningAck timeout for user %s. Read loop might be stalled.", uc.UserId))
		}
	})

	// The main read loop for the WebSocket connection
	for {
		// Update the goroutine tracker's last active time
		z.UpdateLastActive(time.Now(), gor.LastActiveInfo{
			Info: vbu.JoinArgs(
				"ef36f2a4-b1e0-4411-af2e-9252844ed314",
				fmt.Sprintf("handling ws messages for user %s, device %s", uc.UserId, uc.DeviceId),
			),
		})

		// Signal that we are about to start reading messages.
		// This signal is sent once right before the read loop truly begins blocking on ReadMessage.
		// Use a non-blocking send (`select` with a default or a done channel) to ensure it's sent only once.
		select {
		case readStartSignal <- struct{}{}: // Send signal
			// Signal sent successfully (only happens on the first iteration)
		default:
			// Signal already sent, do nothing. Or if channel is full (shouldn't happen with buffer 1)
		}


		if uc.IsConnClosed {
			vbl.Stdout.Warn(vbl.Id("vid/a4295d402f9b"), fmt.Sprintf("WS connection already marked closed for user %s, device %s. Exiting read loop.", uc.UserId, uc.DeviceId))
			// The defer block will handle cleanup
			return // Exit the loop and the goroutine
		}

		// Read the next message from the WebSocket connection
		// This call is blocking until a message is received, an error occurs, or the connection is closed.
		msgType, msg, err := uc.WSConn.ReadMessage()

		// Check again if the connection was marked closed *during* the ReadMessage call
		if uc.IsConnClosed {
			vbl.Stdout.Debug(vbl.Id("vid/ws_read_conn_closed_after"), fmt.Sprintf("ReadMessage returned after connection was marked closed for user %s. Message type: %d, error: %v", uc.UserId, msgType, err))
			// The defer block will handle cleanup, including firing OnDisconnect
			return // Exit the loop
		}

		// Handle errors during ReadMessage
		if err != nil {
			// Common WebSocket errors indicate closure or network issues
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				vbl.Stdout.Warn(vbl.Id("vid/ws_unexpected_close"), fmt.Sprintf("Unexpected WS close error for user %s: %v", uc.UserId, err))
			} else if err == io.EOF {
				vbl.Stdout.Info(vbl.Id("vid/ws_eof"), fmt.Sprintf("EOF received for user %s WS connection: %v", uc.UserId, err))
			} else {
				// Log other potential errors like network read errors
				vbl.Stdout.Error(vbl.Id("vid/ws_read_message_err"), fmt.Sprintf("Error reading WS message for user %s: %v", uc.UserId, err))
			}
			// In case of any read error, assume the connection is no longer viable
			uc.IsConnClosed = true // Mark the connection as closed
			// The defer block will handle cleanup and exit the goroutine
			return // Exit the loop
		}

		// Handle message types other than text/binary (like control messages)
		switch msgType {
		case websocket.PingMessage:
			vbl.Stdout.Debug(vbl.Id("vid/ws_ping_recv"), fmt.Sprintf("Ping received from user %s, sending pong.", uc.UserId))
			// Respond with a pong message, using the received ping payload
			if err := uc.WriteMessageSafely(websocket.PongMessage, msg); err != nil {
				vbl.Stdout.Warn(vbl.Id("vid/ws_pong_send_err"), fmt.Sprintf("Error sending pong to user %s: %v", uc.UserId, err))
				// If pong fails, the connection might be bad, but try to continue for now.
			}
			continue // Continue to the next iteration of the read loop

		case websocket.PongMessage:
			// Optionally handle pong messages, e.g., update a keep-alive timer
			vbl.Stdout.Trace(vbl.Id("vid/ws_pong_recv"), fmt.Sprintf("Pong received from user %s.", uc.UserId))
			continue // Continue to the next iteration

		case websocket.CloseMessage:
			vbl.Stdout.Info(vbl.Id("vid/ws_close_recv"), fmt.Sprintf("Close message received from user %s. Code: %d, Text: %s", uc.UserId, websocket.CloseStatus(msg), string(msg)))
			uc.IsConnClosed = true // Mark the connection as closed based on client's request
			// The defer block will handle cleanup and exit the goroutine
			return // Exit the loop

		case websocket.TextMessage, websocket.BinaryMessage:
			// Process text or binary messages - this is where application logic happens
			// Continue below...

		default:
			// Handle other unexpected message types if necessary
			vbl.Stdout.Warn(vbl.Id("vid/ws_unknown_msg_type"), fmt.Sprintf("Received unknown WS message type %d from user %s", msgType, uc.UserId))
			// Optionally send a close frame or error back to the client
			continue // Continue to the next iteration
		}


		// Process Text/Binary Messages
		if vbu.IsWhitespace(msg) {
			vbl.Stdout.Warn(vbl.Id("vid/fbcb5a167b59"), fmt.Sprintf("Received a message that was total whitespace from user %s", uc.UserId))
			uc.WriteJSONSafely(cs, s.VibeChatErr(
				"9576890d-9486-4594-bd1c-cf62702a989f",
				fmt.Errorf("empty message sent to server"),
			))
			continue // Process next message
		}

		// Assuming messages follow the WSIncomingMsg structure: {Meta: ..., List: [...]}
		var wsx WSIncomingMsg
		if err := easyjson.Unmarshal(msg, &wsx); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/d08c320c4788"), fmt.Sprintf("Error decoding JSON message from user %s: %v, message was: %s", uc.UserId, err, string(msg)))
			uc.WriteJSONSafely(cs, s.VibeChatErr(
				"e1899cd9-163e-42ca-b84a-d4412742b28a",
				fmt.Errorf("error decoding JSON message: '%s'", string(msg)),
			))
			continue // Process next message
		}

		if len(wsx.List) < 1 {
			vbl.Stdout.Info(vbl.Id("vid/a9eed85b0dcc"), fmt.Sprintf("Received WS message with empty List from user %s: %+v", uc.UserId, wsx))
			// Decide if an empty list is an error or just ignorable
			// uc.WriteJSONSafely(cs, s.VibeChatErr(
			// 	"3923dd6f-fe26-4bf3-a0d4-70fc0fb0544b",
			// 	fmt.Errorf("the 'List' property is empty, should have at least one element"),
			// ))
			continue // Process next message
		}

		// Handle each item in the List slice (each item is an individual message/event)
		// Process list items concurrently if they are independent, or serially if order matters.
		// For simplicity and potential dependencies (like offer/answer ordering), let's process serially for now.
		for _, dataMap := range wsx.List { // dataMap is map[string]interface{}

			// Extract "@vibe-type", "@vibe-data", and "@vibe-meta"
			vibeTypeVal, typeOk := dataMap["@vibe-type"]
			vibeDataPayload, dataOk := dataMap["@vibe-data"] // This is interface{}
			vibeMetaVal, metaOk := dataMap["@vibe-meta"]

			if !typeOk {
				vbl.Stdout.Warn(vbl.Id("vid/822f818c0fc0"), fmt.Sprintf("Missing '@vibe-type' key in WS message item from user %s: %+v", uc.UserId, dataMap))
				uc.WriteJSONSafely(cs, s.VibeChatErr("822f818c0fc0", fmt.Errorf("malformed ws message item: missing '@vibe-type'")))
				continue // Skip this item, process the next
			}

			vibeDataTypeAsString, typeStringOk := vibeTypeVal.(string)
			if !typeStringOk {
				vbl.Stdout.Warn(vbl.Id("vid/5963f1ade867"), fmt.Sprintf("The '@vibe-type' key is not a string from user %s: %+v", uc.UserId, dataMap))
				uc.WriteJSONSafely(cs, s.VibeChatErr("5963f1ade867", fmt.Errorf("malformed ws message item: '@vibe-type' is not a string")))
				continue // Skip this item
			}

			if !dataOk {
				// Depending on message type, @vibe-data might be optional.
				// Log a warning but allow processing if the type handler doesn't require data.
				vbl.Stdout.Debug(vbl.Id("vid/2b44b0edcd36"), fmt.Sprintf("Missing optional '@vibe-data' key in WS message item (type: %s) from user %s: %+v", vibeDataTypeAsString, uc.UserId, dataMap))
			}


			// Ensure metaD is a map if present
			var metaD map[string]interface{}
			if metaOk {
				if v, ok := vibeMetaVal.(map[string]interface{}); ok {
					metaD = v
				} else {
					metaD = make(map[string]interface{})
					vbl.Stdout.Warn(vbl.Id("vid/be6e1d9bbbcb"), fmt.Sprintf("Expected '@vibe-meta' to be a map[string]interface{} but it was not, from user %s. Value: %+v", uc.UserId, vibeMetaVal))
				}
			} else {
				metaD = make(map[string]interface{}) // Initialize empty map if meta is missing
			}


			// --- Dispatch based on @vibe-type ---
			// Handle WebRTC signaling separately due to its specific payload structure
			if vibeDataTypeAsString == "webrtc-signaling" {
				// handleWebRTCMessage expects the *content* of @vibe-data (interface{})
				go func() { // Process signaling in a goroutine to keep the read loop fast
					defer func() { // Recover from panics in the signaling handler
						if r := vhp.HandlePanicWithUserCtxFromWebsocket("vid/webrtc_signaling_panic", uc); r != nil {
							vapm.SendTrace("vid/webrtc_signaling_panic_apm", fmt.Sprintf("%v", r))
							vbl.Stdout.Error("vid/webrtc_signaling_panic_log", fmt.Sprintf("%v", r))
							uc.WriteJSONSafely(cs, VibeChatError{ // Send error back to client
								ErrId:      "vid/webrtc_signaling_panic_errid",
								ErrMessage: "Panic/Exception during WebRTC signaling",
							})
						}
					}()
					if err := s.handleWebRTCMessage(uc, vibeDataPayload, metaD, cs); err != nil {
						vbl.Stdout.Error(vbl.Id("vid/webrtc_handle_msg_err"), fmt.Sprintf("Error handling WebRTC message for user %s: %v", uc.UserId, err))
						// Error response is already sent back inside handleWebRTCMessage
					}
				}()
				continue // Process next item in the List immediately

			}


			// Handle other application-specific message types (Mongo operations, etc.)
			// These handlers assume @vibe-data, if present, needs to be unmarshaled into a specific type.
			switch vibeDataTypeAsString {

			case "ConsumeFromRMQ": // Removed functionality, keep case as placeholder or remove
				vbl.Stdout.Warn(vbl.Id("vid/consume_rmq_removed"), fmt.Sprintf("Received 'ConsumeFromRMQ' message from user %s, but RabbitMQ is removed.", uc.UserId))
				uc.WriteJSONSafely(cs, s.VibeChatErr("vid/consume_rmq_removed_err", fmt.Errorf("'ConsumeFromRMQ' functionality is not available.")))
				continue

			case "ConsumeFromRedis": // Removed functionality, keep case as placeholder or remove
				vbl.Stdout.Warn(vbl.Id("vid/consume_redis_removed"), fmt.Sprintf("Received 'ConsumeFromRedis' message from user %s, but Redis is removed.", uc.UserId))
				uc.WriteJSONSafely(cs, s.VibeChatErr("vid/consume_redis_removed_err", fmt.Errorf("'ConsumeFromRedis' functionality is not available.")))
				continue

			case "AddLocation": // Removed functionality, keep case as placeholder or remove
				// TODO: Re-implement if location tracking is needed
				vbl.Stdout.Warn(vbl.Id("vid/add_location_removed"), fmt.Sprintf("Received 'AddLocation' message from user %s, but location handling is removed.", uc.UserId))
				uc.WriteJSONSafely(cs, s.VibeChatErr("vid/add_location_removed_err", fmt.Errorf("'AddLocation' functionality is not available.")))
				continue

			case "AddUserToChat": // Placeholder
				vbl.Stdout.Info(vbl.Id("vid/add_user_to_chat"), fmt.Sprintf("Received 'AddUserToChat' message from user %s.", uc.UserId))
				// TODO: Implement logic to add a user to a chat and update DB/subscriptions
				continue

			case "RemoveUserFromChat": // Placeholder
				vbl.Stdout.Info(vbl.Id("vid/remove_user_from_chat"), fmt.Sprintf("Received 'RemoveUserFromChat' message from user %s.", uc.UserId))
				// TODO: Implement logic to remove a user from a chat and update DB/subscriptions
				continue

			case "AckMessage": // Removed functionality, keep case as placeholder or remove
				// TODO: Re-implement if message acks are handled this way
				vbl.Stdout.Warn(vbl.Id("vid/ack_message_removed"), fmt.Sprintf("Received 'AckMessage' message from user %s, but ack handling is removed.", uc.UserId))
				uc.WriteJSONSafely(cs, s.VibeChatErr("vid/ack_message_removed_err", fmt.Errorf("'AckMessage' functionality is not available.")))
				continue


			case "PollMongoForChatMessagesByConv": // Keep Mongo polling logic
				// This handles requests from the client to fetch messages for specific conversations from MongoDB.
				go func() { // Run in a goroutine to avoid blocking the read loop

					type MongoConvIds struct {
						ConvIds []string `json:"ConvIds"`
					}

					defer func() { // Recover from panic
						if r := vhp.HandlePanicWithUserCtxFromWebsocket("933ed3fc35a1", uc); r != nil {
							vapm.SendTrace("ef01ddc0-87b5-47e7-ac15-9242aee36636", fmt.Sprintf("%v", r))
							vbl.Stdout.Error(vbl.Id("vid/5db49bdfb378"), fmt.Sprintf("%v", r))
							uc.WriteJSONSafely(cs, VibeChatError{
								ErrId:      "cf45462b-48bc-4b3e-a26d-17d8048888e6",
								ErrMessage: "Panic/Exception",
							})
						}
					}()

					var v MongoConvIds
					// Unmarshal from vibeDataPayload (interface{})
					dataBytes, err := json.Marshal(vibeDataPayload)
					if err != nil {
						vbl.Stdout.Error("Cannot marshal 'vibeDataPayload' for PollMongoForChatMessagesByConv:", err)
						uc.WriteJSONSafely(cs, s.VibeChatErr("vid/unmarshal_poll_conv_err", fmt.Errorf("malformed poll data")))
						return
					}
					if err := json.Unmarshal(dataBytes, &v); err != nil {
						vbl.Stdout.Error(vbl.Id("vid/216bf8822722"), fmt.Sprintf("Cannot unmarshal 'MongoConvIds' from user %s: %v, data: %s", uc.UserId, err, string(dataBytes)))
						uc.WriteJSONSafely(cs, s.VibeChatErr("d707630cb0b2", fmt.Errorf("malformed conversation IDs list")))
						return
					}

					var convObjIds = []primitive.ObjectID{} // Convert string IDs to ObjectIDs

					for _, c := range v.ConvIds {
						// make sure conv ids are valid object ids
						if z, err := primitive.ObjectIDFromHex(c); err != nil {
							vbl.Stdout.Error("db465cd4-95c8-4a35-be29-207d01e64160", fmt.Sprintf("Invalid convId '%s' from user %s: %v", c, uc.UserId, err))
							uc.WriteJSONSafely(cs, ErrDeets{
								"90242477-32bc-4424-b9fc-12cd0e2b594b",
								fmt.Sprintf("Invalid conversation ID: %s", c),
								err,
								nil,
							})
							// Decide if you stop processing or skip this invalid ID
							// For now, returning the error stops this goroutine for this request.
							return
						} else if z.IsZero() {
							vbl.Stdout.Error("0d02dbcb-45b1-4b16-a740-e07353d73e84", fmt.Sprintf("Zero object ID for convId '%s' from user %s.", c, uc.UserId))
							uc.WriteJSONSafely(cs, ErrDeets{
								"462c7e45-8952-4e98-8dc6-8ea39f0d8857",
								fmt.Sprintf("Invalid conversation ID (zero ID): %s", c),
								fmt.Errorf("zero object id"),
								nil,
							})
							return
						} else {
							convObjIds = append(convObjIds, z) // Add valid ObjectID
						}
					}

					// Call the function to fetch and send messages for the given conversation IDs
					go func() { // Run the actual fetching/sending in another goroutine
						defer func() { // Recover from panic here too
							if r := vhp.HandlePanicWithUserCtxFromWebsocket("vid/poll_conv_fetch_panic", uc); r != nil {
								vapm.SendTrace("vid/poll_conv_fetch_panic_apm", fmt.Sprintf("%v", r))
								vbl.Stdout.Error("vid/poll_conv_fetch_panic_log", fmt.Sprintf("%v", r))
								uc.WriteJSONSafely(cs, VibeChatError{
									ErrId:      "vid/poll_conv_fetch_panic_errid",
									ErrMessage: "Panic/Exception during message fetch",
								})
							}
						}()
						if err := s.getAllMessagesForParticularChats(uc, convObjIds); err != nil {
							vbl.Stdout.Error(vbl.Id("vid/82857afbb107"), fmt.Sprintf("Error fetching messages for user %s: %v", uc.UserId, err))
							// Error is already sent back inside getAllMessagesForParticularChats
						}
						vbl.Stdout.Info(vbl.Id("vid/poll_conv_fetch_done"), fmt.Sprintf("Finished polling messages for user %s for conversations: %v", uc.UserId, v.ConvIds))
					}()
				}()
				continue // Process next item in the List

			case "PollMongoForAllChatMessages": // Keep Mongo polling logic (all chats)
				// This handles requests from the client to fetch messages for *all* their chats from MongoDB.
				go func() { // Run in a goroutine to avoid blocking the read loop
					defer func() { // Recover from panic
						if r := vhp.HandlePanicWithUserCtxFromWebsocket("e76f13d872d4", uc); r != nil {
							vapm.SendTrace("2c1711ff-3739-4c19-81b2-b7a3d326c8c6", fmt.Sprintf("%v", r))
							vbl.Stdout.Error(vbl.Id("vid/75103e9e7c28"), fmt.Sprintf("%v", r))
							uc.WriteJSONSafely(cs, VibeChatError{
								ErrId:      "95f16585-5277-4291-946e-dc68b6d5e45a",
								ErrMessage: "Panic/Exception",
							})
						}
					}()
					// Call the function to fetch and send messages for all chats involving this user
					if err := s.getMongoMessagesForAllChatsByUser(cs, uc); err != nil {
						vbl.Stdout.Error(vbl.Id("vid/742d48df0046"), fmt.Sprintf("Error fetching all messages for user %s: %v", uc.UserId, err))
						// Error is already sent back inside getMongoMessagesForAllChatsByUser
					}
					vbl.Stdout.Info(vbl.Id("vid/poll_all_fetch_done"), fmt.Sprintf("Finished polling all messages for user %s.", uc.UserId))
				}()
				continue // Process next item in the List

			case "ChatIsOver": // Placeholder
				vbl.Stdout.Info(vbl.Id("vid/chat_is_over"), fmt.Sprintf("Received 'ChatIsOver' message from user %s.", uc.UserId))
				// TODO: Implement logic when a chat ends (e.g., unsubscribe, clean up server resources)
				continue

			case "PollForKafkaMessages": // Removed functionality, keep case as placeholder or remove
				vbl.Stdout.Warn(vbl.Id("vid/poll_kafka_removed"), fmt.Sprintf("Received 'PollForKafkaMessages' message from user %s, but Kafka is removed.", uc.UserId))
				uc.WriteJSONSafely(cs, s.VibeChatErr("vid/poll_kafka_removed_err", fmt.Errorf("'PollForKafkaMessages' functionality is not available.")))
				continue

			case "MongoChatMessage": // Keep handling new chat messages
				// This handles new chat messages sent by a client over the WebSocket.
				go func() { // Run in a goroutine to avoid blocking the read loop

					defer func() { // Recover from panic
						if r := vhp.HandlePanicWithUserCtxFromWebsocket("651c85a6bf82", uc); r != nil {
							vbl.Stdout.Error(vbl.Id("vid/0f0495a7399e"), fmt.Sprintf("%v", r))
							uc.WriteJSONSafely(cs, s.VibeChatErr(
								"4e99f5f6-7f7c-4453-9f7c-c1a97c3e8358",
								fmt.Errorf("unknown error caught by recover()"),
							))
						}
					}()

					var v mngo_types.MongoChatMessage
					// Unmarshal from vibeDataPayload (interface{})
					dataBytes, err := json.Marshal(vibeDataPayload)
					if err != nil {
						vbl.Stdout.Error("Cannot marshal 'vibeDataPayload' for MongoChatMessage:", err)
						uc.WriteJSONSafely(cs, s.VibeChatErr("vid/unmarshal_mongo_chat_msg_err", fmt.Errorf("malformed message data")))
						return
					}
					if err := easyjson.Unmarshal(dataBytes, &v); err != nil {
						vbl.Stdout.Error(vbl.Id("vid/0443df2b6e74"), fmt.Sprintf("Cannot unmarshal 'MongoChatMessage' from user %s: %v, data: %s", uc.UserId, err, string(dataBytes)))
						uc.WriteJSONSafely(cs, s.VibeChatErr(
							"c0241c18-a211-450e-85c3-36b7e813733f",
							fmt.Errorf("could not cast message to desired backend struct: '%v'", string(dataBytes)),
						))
						return
					}

					// Validate necessary fields in the message
					if v.Id.IsZero() {
						errId := "00c11494-d257-4ab0-9c03-087a52f9e912"
						errMessage := fmt.Sprintf("missing message object id: %v", v)
						vapm.SendTrace(errId, errMessage)
						vbl.Stdout.Error(errId, errMessage)
						uc.WriteJSONSafely(cs, VibeChatError{errId, errMessage})
						return
					}
					if v.ChatId.IsZero() {
						errId := "60df47eb-a903-4f70-a0b4-eca527db61ff"
						errMessage := fmt.Sprintf("missing (zero) 'ChatId' in message: '%v'", v)
						vbl.Stdout.Error(errId, errMessage)
						vapm.SendTrace(errId, errMessage)
						uc.WriteJSONSafely(cs, VibeChatError{errId, errMessage})
						return
					}
					if v.CreatedByUserId.IsZero() {
						errId := "vid/mongo_chat_msg_missing_author"
						errMessage := fmt.Sprintf("missing 'CreatedByUserId' in message: '%v'", v)
						vbl.Stdout.Error(errId, errMessage)
						vapm.SendTrace(errId, errMessage)
						uc.WriteJSONSafely(cs, VibeChatError{errId, errMessage})
						return
					}
					// Ensure the sender ID matches the user ID associated with this connection
					if v.CreatedByUserId.Hex() != uc.UserObjId.Hex() {
						errId := "vid/mongo_chat_msg_author_mismatch"
						errMessage := fmt.Sprintf("message author mismatch: expected %s, got %s", uc.UserObjId.Hex(), v.CreatedByUserId.Hex())
						vbl.Stdout.Warn(errId, errMessage)
						// Decide how to handle this security/logic issue. Deny the message?
						uc.WriteJSONSafely(cs, VibeChatError{errId, errMessage})
						return
					}


					// --- Message Handling Logic ---
					// 1. Save to MongoDB
					// 2. Relay to other connected clients in the same conversation over WS

					// Save to MongoDB
					go func() { // Run DB operation concurrently
						defer func() {
							if r := recover(); r != nil {
								vbl.Stdout.Error(vbl.Id("vid/mongo_insert_msg_panic"), fmt.Sprintf("%v", r))
								vapm.SendTrace("vid/mongo_insert_msg_panic_apm", fmt.Sprintf("%v", r))
								// No need to send error back here, main handler does it
							}
						}()
						vbl.Stdout.Info(vbl.Id("vid/mongo_insert_msg"), fmt.Sprintf("Attempting to save new chat message '%s' to mongo for conv '%s'", v.Id.Hex(), v.ChatId.Hex()))
						coll := s.M.Col.ChatConvMessages // Assuming collection is available
						if _, err := s.M.DoInsertOne(vctx.NewStdMgoCtx(), coll, v); err != nil {
							vapm.SendTrace("4c4a70d4-6fef-42ab-9747-1510cd789955", fmt.Sprintf("Mongo insert error: %v", err))
							vbl.Stdout.Error(vbl.Id("vid/26cd003aa2e9"), fmt.Sprintf("Error inserting chat message into Mongo: %v", err))
							// Decide if you send an error back to the client if DB save fails
							// uc.WriteJSONSafely(cs, s.VibeChatErr("mongo_insert_msg_err", fmt.Errorf("failed to save message")))
						} else {
							vbl.Stdout.Debug(vbl.Id("vid/mongo_insert_msg_success"), "Successfully wrote new chat message to mongo")
							// Send an acknowledgement back to the *sender* client
							// Use the specific WsClientAck type or a similar structure
							ackPayload := map[string]interface{}{
								"ack": true,
								"reason": "mongo received",
								"convId": v.ChatId.Hex(),
								"messageId": v.Id.Hex(),
							}
							// Wrap in WebsocketOutgoingPayload and send via WriteJSONSafely
							outgoingAck := WebsocketOutgoingPayload{
								Meta: WSOutgoingMeta{FromSource: "Server", TimeCreated: time.Now().UTC().String()},
								Messages: []interface{}{map[string]interface{}{"@vibe-type": "message-ack", "@vibe-data": ackPayload}}, // Example structure
							}
							if ackErr := uc.WriteJSONSafely(cs, outgoingAck); ackErr != nil {
								vibelog.Stdout.Warn(vibelog.Id("vid/ws_send_ack_err"), fmt.Sprintf("Error sending message ack to sender %s: %v", uc.UserId, ackErr))
							} else {
								vibelog.Stdout.Debug(vibelog.Id("vid/ws_send_ack_success"), fmt.Sprintf("Sent message ack to sender %s for message %s", uc.UserId, v.Id.Hex()))
							}
						}
					}()


					// Relay message to other connected clients in the same conversation over WS
					go func() { // Run relaying concurrently
						defer func() { // Recover from panic during relay
							if r := recover(); r != nil {
								vbl.Stdout.Error(vbl.Id("vid/ws_relay_msg_panic"), fmt.Sprintf("%v", r))
								vapm.SendTrace("vid/ws_relay_msg_panic_apm", fmt.Sprintf("%v", r))
								// No need to send error back here
							}
						}()

						convIdStr := v.ChatId.Hex()
						s.mu.Lock() // Protect the convIdToConns map while reading it
						connsInConv := s.convIdToConns[convIdStr] // Get map of connections for this conversation
						s.mu.Unlock() // Release lock quickly

						if connsInConv != nil && len(connsInConv) > 0 {
							vbl.Stdout.Debug(vbl.Id("vid/ws_relay_msg_start"), fmt.Sprintf("Relaying message %s to %d connections in conversation %s", v.Id.Hex(), len(connsInConv), convIdStr))

							// Prepare the message payload to relay. It should include the message data.
							// Use a structure the client's WS handler expects for new chat messages.
							// Example: map with type and data
							relayMsgPayload := map[string]interface{}{"@vibe-type": "new-chat-message", "@vibe-data": v}

							// Wrap in the standard outgoing payload structure
							outgoingRelayPayload := WebsocketOutgoingPayload{
								Meta: WSOutgoingMeta{FromSource: "Server-Relay", TimeCreated: time.Now().UTC().String()},
								Messages: []interface{}{relayMsgPayload},
							}

							// Iterate over connections in the conversation and send the message
							// Iterate over a copy of the map keys or use a goroutine per send to avoid blocking
							connKeys := make([]*vuc.UserCtx, 0, len(connsInConv))
							for k := range connsInConv {
								connKeys = append(connKeys, k)
							}

							for _, otherUc := range connKeys {
								// Do not send the message back to the original sender's connection
								if otherUc != uc { // Compare pointers
									go func(targetUc *vuc.UserCtx) { // Send concurrently to each recipient
										defer func() { // Recover from panics within recipient goroutine
											if r := recover(); r != nil {
												vbl.Stdout.Error(vbl.Id("vid/ws_relay_recipient_panic"), fmt.Sprintf("Panic sending relayed message to user %s: %v", targetUc.UserId, r))
											}
										}()
										vbl.Stdout.Debug(vbl.Id("vid/ws_relay_sending"), fmt.Sprintf("Sending relayed message %s to user %s", v.Id.Hex(), targetUc.UserId))
										if err := targetUc.WriteJSONSafely(cs, outgoingRelayPayload); err != nil {
											// Log error if sending fails for a specific peer (e.g., they disconnected)
											// The cleanup of disconnected peers is handled elsewhere (defer in handleIncomingWSMessages)
											vbl.Stdout.Warn(vbl.Id("vid/ws_relay_write_err"), fmt.Sprintf("Error sending relayed message to user %s: %v", targetUc.UserId, err))
										}
									}(otherUc) // Pass the UserCtx pointer
								}
							}
							vbl.Stdout.Debug(vbl.Id("vid/ws_relay_msg_done"), fmt.Sprintf("Finished dispatching relayed message %s for conversation %s.", v.Id.Hex(), convIdStr))

						} else {
							vbl.Stdout.Debug(vbl.Id("vid/ws_relay_msg_no_conns"), fmt.Sprintf("No other connections found in conversation %s to relay message %s.", convIdStr, v.Id.Hex()))
						}
					}()

					// Return nil if initial unmarshal and basic checks passed.
					// Errors during DB save or relaying are handled and logged within their goroutines.
					// If you require strict confirmation that the message was received by *all* currently
					// connected peers before acknowledging the sender, this logic needs to be more complex.
					// The current approach acknowledges the sender once the message is saved to the DB.

				})
				continue // Process next item in the List


			case "MongoChatConv": // Keep handling new chat conversations
				// This handles new chat conversation creation requests from a client.
				go func() { // Run in a goroutine

					defer func() { // Recover from panic
						if r := vhp.HandlePanicWithUserCtxFromWebsocket("f9a427d23751", uc); r != nil {
							vbl.Stdout.Error(vbl.Id("vid/d99e59f9dbbf"), fmt.Sprintf("%v", r))
							uc.WriteJSONSafely(cs, s.VibeChatErr("fa771564-9263-409b-b376-3513d43eb3dd", fmt.Errorf("Panic/Exception")))
						}
					}()

					var v mngo_types.MongoChatConv
					// Unmarshal from vibeDataPayload (interface{})
					dataBytes, err := json.Marshal(vibeDataPayload)
					if err != nil {
						vbl.Stdout.Error("Cannot marshal 'vibeDataPayload' for MongoChatConv:", err)
						uc.WriteJSONSafely(cs, s.VibeChatErr("vid/unmarshal_mongo_chat_conv_err", fmt.Errorf("malformed conversation data")))
						return
					}

					if err := easyjson.Unmarshal(dataBytes, &v); err != nil {
						vbl.Stdout.Error(vbl.Id("vid/e69d82d5c85a"), fmt.Sprintf("Cannot unmarshal 'MongoChatConv' from user %s: %v, data: %s", uc.UserId, err, string(dataBytes)))
						uc.WriteJSONSafely(cs, s.VibeChatErr("d178cfc1-03dc-4b5e-b255-155280d198c6", err))
						return
					}

					// TODO: Add validation for the new conversation object (e.g., participant IDs are valid)

					// --- Conversation Handling Logic ---
					// 1. Save to MongoDB
					// 2. Notify relevant users (participants in the new chat) over WS

					// Save to MongoDB
					go func() { // Run DB operation concurrently
						defer func() { // Recover from panic
							if r := recover(); r != nil {
								vbl.Stdout.Error(vbl.Id("vid/mongo_insert_conv_panic"), fmt.Sprintf("%v", r))
								vapm.SendTrace("vid/mongo_insert_conv_panic_apm", fmt.Sprintf("%v", r))
								// No need to send error back here
							}
						}()
						vbl.Stdout.Info(vbl.Id("vid/mongo_insert_conv"), fmt.Sprintf("Attempting to save new chat conversation '%s' to mongo.", v.Id.Hex()))
						coll := s.M.Col.ChatConversations // Assuming collection is available
						if _, err := s.M.DoInsertOne(vctx.NewStdMgoCtx(), coll, v); err != nil {
							vapm.SendTrace("vid/mongo_insert_conv_err_apm", fmt.Sprintf("Mongo insert error: %v", err))
							vbl.Stdout.Error(vbl.Id("vid/mongo_insert_conv_err"), fmt.Sprintf("Error inserting chat conversation into Mongo: %v", err))
							// Decide if you send an error back to the client if DB save fails
							// uc.WriteJSONSafely(cs, s.VibeChatErr("mongo_insert_conv_err", fmt.Errorf("failed to save conversation")))
						} else {
							vbl.Stdout.Debug(vbl.Id("vid/mongo_insert_conv_success"), "Successfully wrote new chat conversation to mongo")
							// Notify the sender about the successful creation?

							// Notify all participants about the new conversation
							go func() { // Run notification concurrently
								defer func() { // Recover from panic
									if r := recover(); r != nil {
										vbl.Stdout.Error(vbl.Id("vid/ws_notify_conv_panic"), fmt.Sprintf("%v", r))
										vapm.SendTrace("vid/ws_notify_conv_panic_apm", fmt.Sprintf("%v", r))
									}
								}()
								vbl.Stdout.Debug(vbl.Id("vid/ws_notify_conv_start"), fmt.Sprintf("Notifying participants of new conversation %s.", v.Id.Hex()))

								// Prepare the message payload to notify clients about the new conversation
								// Include the conversation data.
								notifyMsgPayload := map[string]interface{}{"@vibe-type": "new-chat-conversation", "@vibe-data": v}
								outgoingNotifyPayload := WebsocketOutgoingPayload{
									Meta: WSOutgoingMeta{FromSource: "Server-Notification", TimeCreated: time.Now().UTC().String()},
									Messages: []interface{}{notifyMsgPayload},
								}

								// Find all UserCtx for users in v.ParticipantUserIds and send them the message
								s.mu.Lock() // Protect userIdToConns map
								// Iterate over participant user IDs
								for _, participantID := range v.ParticipantUserIds {
									participantIDHex := participantID.Hex()
									// Find all connections for this participant user ID
									if conns, ok := s.userIdToConns[participantIDHex]; ok && conns != nil {
										// Iterate over each connection for this user and send the notification
										for _, targetUc := range conns {
											go func(tu *vuc.UserCtx) { // Send concurrently to each connection
												defer func() { if r := recover(); r != nil { vbl.Stdout.Error(vbl.Id("vid/ws_notify_recipient_panic"), fmt.Sprintf("Panic sending notification to user %s conn: %v", tu.UserId, r)) } }()
												vbl.Stdout.Debug(vbl.Id("vid/ws_notify_sending"), fmt.Sprintf("Sending new conversation notification %s to user %s connection.", v.Id.Hex(), tu.UserId))
												if err := tu.WriteJSONSafely(cs, outgoingNotifyPayload); err != nil {
													vbl.Stdout.Warn(vbl.Id("vid/ws_notify_write_err"), fmt.Sprintf("Error sending new conversation notification to user %s connection: %v", tu.UserId, err))
												}
											}(targetUc)
										}
										vbl.Stdout.Debug(vbl.Id("vid/ws_notify_user_done"), fmt.Sprintf("Finished dispatching new conversation notification %s for user %s.", v.Id.Hex(), participantIDHex))
									} else {
										vbl.Stdout.Debug(vbl.Id("vid/ws_notify_user_no_conns"), fmt.Sprintf("No active connections found for participant %s to notify about new conversation %s.", participantIDHex, v.Id.Hex()))
									}
								}
								s.mu.Unlock() // Release lock

								vbl.Stdout.Debug(vbl.Id("vid/ws_notify_conv_done"), fmt.Sprintf("Finished notifying all participants of new conversation %s.", v.Id.Hex()))
							}()
						}
					}()


				}()
				continue // Process next item in the List


			case "MongoChatUser": // Keep handling new chat users (placeholder)
				// This handles new chat user creation requests from a client (placeholder).
				go func() { // Run in a goroutine

					defer func() { // Recover from panic
						if r := vhp.HandlePanicWithUserCtxFromWebsocket("cd172d55afae", uc); r != nil {
							vbl.Stdout.Error(vbl.Id("vid/c367a623c9f1"), fmt.Sprintf("%v", r))
							uc.WriteJSONSafely(cs, s.VibeChatErr("7666814d-5353-4477-9856-6e7cb4aae58c", fmt.Errorf("Panic/Exception")))
						}
					}()

					var v mngo_types.MongoChatUser
					// Unmarshal from vibeDataPayload (interface{})
					dataBytes, err := json.Marshal(vibeDataPayload)
					if err != nil {
						vbl.Stdout.Error("Cannot marshal 'vibeDataPayload' for MongoChatUser:", err)
						uc.WriteJSONSafely(cs, s.VibeChatErr("vid/unmarshal_mongo_chat_user_err", fmt.Errorf("malformed user data")))
						return
					}

					if err := easyjson.Unmarshal(dataBytes, &v); err != nil {
						vbl.Stdout.Error(vbl.Id("vid/37e1b2f21f35"), fmt.Sprintf("Cannot unmarshal 'MongoChatUser' from user %s: %v, data: %s", uc.UserId, err, string(dataBytes)))
						uc.WriteJSONSafely(cs, s.VibeChatErr("e75b9fd1-a6f5-439d-8d46-d31cd1536ad2", err))
						return
					}

					// TODO: Implement logic to save the new user to MongoDB and potentially notify others.
					vbl.Stdout.Info(vbl.Id("vid/mongo_chat_user"), fmt.Sprintf("Received new chat user data from user %s: %+v", uc.UserId, v))


				}()
				continue // Process next item in the List


			default:
				// Handle unknown message types
				vbl.Stdout.Error(vbl.Id("vid/d68449448b93"), fmt.Sprintf("No matching handler for vibe-type '%s' from user %s", vibeDataTypeAsString, uc.UserId))
				uc.WriteJSONSafely(cs, s.VibeChatErr(
					"057c421f-c1dd-4d3a-abc4-4f3057426271",
					fmt.Errorf("no matching handler for vibe-type: '%s'", vibeDataTypeAsString),
				))
				continue // Process next item in the List
			}

		} // End of for loop iterating over wsx.List

	} // End of main read loop (for {})
	// The goroutine exits here when ReadMessage returns an error or a close message is received.

	// Cleanup code outside the loop is handled by the initial `defer` block.
}

// Removed consumeFromKafkaTopic and consumeFromKafkaTopic_OLD

// Removed publishMessageToKafkaTopic

// Removed publishMessageToRabbitWOConfirmation and publishMessageToRabbit

// Removed MyChannel struct

// Removed consumeFromRedisChannel

// writeToUserTopic sends a message (wrapped in WebsocketOutgoingPayload) to all active connections for a specific user.
// This is used for messages intended for a specific user across all their devices/connections.
func (s *Service) writeToUserTopic(callTrace []string, userId string, m interface{}) {
	s.mu.Lock() // Protect the userIdToConns map
	conns := s.userIdToConns[userId]
	s.mu.Unlock() // Release lock

	if len(conns) == 0 {
		vbl.Stdout.Debug(vbl.Id("vid/ws_write_user_no_conns"), fmt.Sprintf("No active connections found for user %s to write to.", userId))
		return
	}

	var cs = callTrace // Use the provided callTrace

	// Prepare the outgoing payload (wrapper for the actual message)
	outgoingPayload := WebsocketOutgoingPayload{
		Meta: WSOutgoingMeta{FromSource: "User-Topic", TimeCreated: time.Now().UTC().String()},
		Messages: []interface{}{m}, // The message 'm' is included in the list
	}

	// Iterate over all connections for this user and send the message concurrently
	for _, uc := range conns {
		go func(targetUc *vuc.UserCtx) { // Send concurrently to avoid blocking
			defer func() { // Recover from panic in sender goroutine
				if r := recover(); r != nil {
					vbl.Stdout.Error(vbl.Id("vid/ws_write_user_send_panic"), fmt.Sprintf("Panic sending user message to user %s conn: %v", targetUc.UserId, r))
				}
			}()
			vbl.Stdout.Debug(vbl.Id("vid/ws_write_user_sending"), fmt.Sprintf("Sending user message to user %s connection.", targetUc.UserId))
			if err := targetUc.WriteJSONSafely(cs, outgoingPayload); err != nil {
				// Log error if sending fails for a specific peer (e.g., they disconnected).
				// Cleanup of disconnected peers is handled elsewhere (defer in handleIncomingWSMessages).
				vbl.Stdout.Warn(vbl.Id("vid/ws_write_user_write_err"), fmt.Sprintf("Error sending user message to user %s connection: %v", targetUc.UserId, err))
			}
		}(uc) // Pass the UserCtx pointer
	}
	vbl.Stdout.Debug(vbl.Id("vid/ws_write_user_done"), fmt.Sprintf("Finished dispatching user message to %d connections for user %s.", len(conns), userId))
}

// writeToConvTopic sends a message (wrapped in WebsocketOutgoingPayload) to all active connections within a specific conversation.
// This is used for messages intended for all participants currently connected to a chat room.
func (s *Service) writeToConvTopic(callTrace []string, convId string, m interface{}) {
	s.mu.Lock() // Protect the convIdToConns map
	conns := s.convIdToConns[convId]
	s.mu.Unlock() // Release lock

	if len(conns) == 0 {
		vbl.Stdout.Debug(vbl.Id("vid/ws_write_conv_no_conns"), fmt.Sprintf("No active connections found for conversation %s to write to.", convId))
		return
	}

	var cs = callTrace // Use the provided callTrace
	vbl.Stdout.Info(vbl.Id("vid/2bf02dc13d6c"), fmt.Sprintf("convIdToConns map size: %d", len(s.convIdToConns)))
	vbl.Stdout.Info(vbl.Id("vid/4e17f1e5f662"), fmt.Sprintf("Writing to conv id: %s, connections: %d", convId, len(conns)))


	// Prepare the outgoing payload (wrapper for the actual message)
	outgoingPayload := WebsocketOutgoingPayload{
		Meta: WSOutgoingMeta{FromSource: "Conv-Topic", TimeCreated: time.Now().UTC().String()},
		Messages: []interface{}{m}, // The message 'm' is included in the list
	}

	// Iterate over all connections for this conversation and send the message concurrently
	for _, uc := range conns {
		go func(targetUc *vuc.UserCtx) { // Send concurrently to avoid blocking
			defer func() { // Recover from panic in sender goroutine
				if r := recover(); r != nil {
					vbl.Stdout.Error(vbl.Id("vid/ws_write_conv_send_panic"), fmt.Sprintf("Panic sending conv message to user %s conn in conv %s: %v", targetUc.UserId, convId, r))
				}
			}()
			vbl.Stdout.Debug(vbl.Id("vid/ws_write_conv_sending"), fmt.Sprintf("Sending conv message to user %s connection in conv %s.", targetUc.UserId, convId))

			if err := targetUc.WriteJSONSafely(cs, outgoingPayload); err != nil {
				// Log error if sending fails for a specific peer (e.g., they disconnected).
				// The cleanup of disconnected peers is handled elsewhere (defer in handleIncomingWSMessages).
				vbl.Stdout.Warn(vbl.Id("vid/ws_write_conv_write_err"), fmt.Sprintf("Error sending conv message to user %s connection in conv %s: %v", targetUc.UserId, convId, err))
			}
		}(uc) // Pass the UserCtx pointer
	}
	vbl.Stdout.Debug(vbl.Id("vid/ws_write_conv_done"), fmt.Sprintf("Finished dispatching conv message to %d connections for conversation %s.", len(conns), convId))
}

// Removed doAudit placeholder (it was already marked as returning nil)
// Removed RabbitMessage struct

// killOldSubs should be adapted to prune connections that are no longer valid if needed,
// but the primary cleanup is now tied to connection read errors/close messages.
func (s *Service) killOldSubs() {
	// TODO: Adapt or remove this function.
	// With brokers removed, this would typically monitor the UserCtx map for connections
	// that seem stalled or inactive (e.g., no ping/pong responses) and trigger SafeClose on them.
	return // Disabled for now
	/*
		s.killOnce.Do(func() {
			go func() {
				for {
					time.Sleep(time.Minute * 5) // Check periodically
					now := time.Now()

	                s.mu.Lock() // Protect maps while iterating and potentially deleting
	                // Create copies of keys to iterate safely while modifying the map
	                userIdsToCheck := make([]string, 0, len(s.userIdToConns))
	                for userId := range s.userIdToConns {
	                    userIdsToCheck = append(userIdsToCheck, userId)
	                }

	                // Iterate users and their connections
	                for _, userId := range userIdsToCheck {
	                    if conns, ok := s.userIdToConns[userId]; ok {
	                        connPtrsToCheck := make([]*vuc.UserCtx, 0, len(conns))
	                        for connPtr := range conns {
	                             connPtrsToCheck = append(connPtrsToCheck, connPtr)
	                        }

	                        for _, uc := range connPtrsToCheck {
	                            // Implement logic to check if connection is old/inactive
	                            // e.g., check a 'last active' timestamp on the UserCtx or lack of recent ping/pongs
	                            // This requires adding such tracking to UserCtx and the ping/pong handlers.
	                            // For demonstration, let's assume uc.IsConnClosed is the only check needed here.
	                            if uc.IsConnClosed {
	                                 // Connection is already marked for closure, the defer in its read loop handles cleanup.
	                                 // No action needed here except perhaps logging if a zombie is found.
	                                 vbl.Stdout.Debug("vid/kill_subs_zombie", fmt.Sprintf("Found marked-closed connection for user %s, device %s. Cleanup should happen soon.", uc.UserId, uc.DeviceId))
	                                 // The defer block in handleIncomingWSMessages handles removing from maps.
	                            } else {
	                                // Connection is still considered active by its handler.
	                                // Check for inactivity? Requires last active timestamp.
	                                // If uc.LastActive.Add(timeout).Before(now) {
	                                //     vbl.Stdout.Warn("vid/kill_subs_inactive", fmt.Sprintf("Connection for user %s, device %s seems inactive. Forcing close.", uc.UserId, uc.DeviceId))
	                                //     uc.SafeClose(false) // Force close inactive connection
	                                // }
	                            }
	                        }
	                    }
	                }
	                s.mu.Unlock() // Release lock
				}
			}()
		})
	*/
}


// Removed broadcastMessage (was likely used for broker-based broadcasting)

// Removed unsubscribeFromQueue and unsubscribeFromQueueAndCloseChannel

// SafeDisconnect struct definition (copied from previous diff)
// easyjson:json
type SafeDisconnect struct {
	FromServer bool `json:"fromServer"`
	Action     string `json:"action"`
}

// gracefulShutdown sends a shutdown message to all active WebSocket connections.
func (s *Service) gracefulShutdown(callTrace []string) {
	var cs = callTrace // Use the provided callTrace

	s.mu.Lock() // Protect the userIdToConns map
	// Create a flat list of all active UserCtx pointers to send messages to,
	// allowing the mutex to be released quickly.
	allActiveUCs := make([]*vuc.UserCtx, 0)
	for _, conns := range s.userIdToConns {
		// Iterate over a copy of the inner map keys to be safe if a connection closes during iteration
		connPtrs := make([]*vuc.UserCtx, 0, len(conns))
		for connPtr := range conns {
			connPtrs = append(connPtrs, connPtr)
		}
		allActiveUCs = append(allActiveUCs, connPtrs...)
	}
	s.mu.Unlock() // Release mutex


	vbl.Stdout.Info(vbl.Id("vid/graceful_shutdown_start"), fmt.Sprintf("Sending graceful shutdown message to %d active connections.", len(allActiveUCs)))

	// Prepare the shutdown message wrapped in the standard outgoing payload
	shutdownMsgPayload := map[string]interface{}{"action": "safe_disconnect"}
	outgoingShutdown := WebsocketOutgoingPayload{
		Meta: WSOutgoingMeta{FromSource: "Server-Shutdown", TimeCreated: time.Now().UTC().String()},
		Messages: []interface{}{map[string]interface{}{"@vibe-type": "server-command", "@vibe-data": shutdownMsgPayload}}, // Example structure
	}


	// Send the shutdown message to each active connection concurrently
	for _, uc := range allActiveUCs {
		go func(targetUc *vuc.UserCtx) {
			defer func() { // Recover from panic in sender goroutine
				if r := recover(); r != nil {
					vbl.Stdout.Error(vbl.Id("vid/graceful_shutdown_send_panic"), fmt.Sprintf("Panic sending shutdown message to user %s conn: %v", targetUc.UserId, r))
				}
			}()
			vbl.Stdout.Debug(vbl.Id("vid/graceful_shutdown_ws"), fmt.Sprintf("Sending graceful shutdown message to user %s connection.", targetUc.UserId))
			// Set ignoreError to true because the connection might already be closing during shutdown
			if err := targetUc.WriteJSONSafely(callTrace, outgoingShutdown); err != nil {
				// Log errors but don't stop the shutdown process
				vbl.Stdout.Warn(vbl.Id("vid/graceful_shutdown_write_err"), fmt.Sprintf("Error sending graceful shutdown message to user %s connection: %v", targetUc.UserId, err))
			}
			// Note: uc.SafeClose(true) is called by the defer in handleIncomingWSMessages when it exits.
			// Sending the close message first is the graceful part.
		}(uc) // Pass the UserCtx pointer
	}

	vbl.Stdout.Info(vbl.Id("vid/graceful_shutdown_end"), "Finished dispatching graceful shutdown messages.")

}


// ErrDeets struct definition (copied from previous diff)
// easyjson:json
type ErrDeets struct {
	ErrId      string `json:"errId"`
	ErrMessage string `json:"errMessage"`
	Error      error `json:"-"` // Exclude the original error object from JSON
	Details    []interface{} `json:"details,omitempty"` // Omit if empty
}

// MarshalJSON implements easyjson.Marshaler.
func (e *ErrDeets) MarshalJSON() ([]byte, error) {
	type Alias ErrDeets // Use an alias to avoid infinite recursion
	return json.Marshal(&struct {
		*Alias
		ErrorString string `json:"error,omitempty"` // Include error string if present
	}{
		Alias: (*Alias)(e),
		ErrorString: func() string {
			if e.Error != nil {
				return e.Error.Error()
			}
			return ""
		}(),
	})
}


// NewErrDeets creates a new ErrDeets instance.
func NewErrDeets(errId string, errMessage string, err error, details []interface{}) *ErrDeets {
	return &ErrDeets{
		ErrId:      errId,
		ErrMessage: errMessage,
		Error:      err,
		Details:    details,
	}
}


// WebSocket related structures (copied or kept)

// getUpgrader creates and configures a WebSocket upgrader.
func getUpgrader() *websocket.Upgrader {
	// TODO: reuse upgrader or create new one per connection if needed
	return &websocket.Upgrader{
		ReadBufferSize:  3049, // Adjust buffer sizes based on expected message sizes
		WriteBufferSize: 3049,
		CheckOrigin: func(r *http.Request) bool {
			// TODO: Implement proper origin checking in production
			// Allow all origins for now
			return true
		},
	}
}

var cfg = virl_conf.GetConf() // Get global config
var upgrader = getUpgrader() // Create a single upgrader instance

func init() {
	// This init function might be called before main's config is fully parsed if imported early.
	// It's better to rely on the config being available in NewService or ServeHTTP.
	// vbl.Stdout.Info(vbl.Id("vid/c119943eac72"), "service file inited.")
}


// ServeHTTP handles incoming HTTP requests, upgrading valid ones to WebSocket connections.
// This is the entry point for new WebSocket connections.
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Set standard CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*") // TODO: Restrict in production
	w.Header().Set("Access-Control-Allow-Methods", "*") // Allow common methods
	w.Header().Set("Access-Control-Allow-Headers", "*") // Allow common headers
	w.Header().Set("Content-Type", "application/json") // Default content type for non-WS responses

	vbl.Stdout.Debug(vbl.Id("vid/ws_serve_http_path"), fmt.Sprintf("Received HTTP request on path: %s", r.URL.Path))

	// Handle health check endpoint separately
	if strings.HasPrefix(r.URL.Path, "/v1/health/3001") { // Assuming 3001 is the WSS port
		vbl.Stdout.Debug(vbl.Id("vid/ws_health_check"), "Responding to WSS health check.")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"Status":"OK"}`)); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/ws_health_check_write_err"), fmt.Sprintf("Error writing health check response: %v", err))
		}
		return
	}

	// Handle OPTIONS requests for preflight checks
	if r.Method == http.MethodOptions {
		vbl.Stdout.Debug(vbl.Id("vid/ws_options"), "Handling OPTIONS request.")
		w.WriteHeader(http.StatusOK) // Respond OK to preflight
		return
	}


	// --- WebSocket Upgrade Process ---
	// This attempts to upgrade the standard HTTP connection to a WebSocket connection.
	// If successful, the function below will block handling messages on the new connection.
	// If unsuccessful (e.g., not a websocket request), it responds with an error.

	// TODO: Implement robust authentication and authorization here before upgrading.
	// Get auth details from headers (JWT, UserID, DeviceID)
	jwtToken := r.Header.Get("x-vibe-jwt-token")
	userID := r.Header.Get("x-vibe-user-id") // Should correspond to the user initiating the connection
	deviceId := r.Header.Get("x-vibe-device-id") // Should be a unique ID for the device


	if false { // TODO: Enable JWT check in production
		if jwtToken == "" {
			vbl.Stdout.Warn("vid/ws_auth_missing_jwt", "Missing x-vibe-jwt-token header.")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(s.VibeChatErr("vid/ws_auth_missing_jwt_err", fmt.Errorf("authentication token missing")))
			return
		}
		// Validate JWT here
		var claimsMap *vjwt.CustomClaims
		cm, err := vjwt.ReadJWT(jwtToken) // Assuming ReadJWT is available and validates token
		if err != nil {
			vbl.Stdout.Warn(vbl.Id("vid/ws_auth_invalid_jwt"), fmt.Sprintf("Invalid JWT token: %v", err))
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(s.VibeChatErr("vid/ws_auth_invalid_jwt_err", fmt.Errorf("invalid authentication token")))
			return
		}
		claimsMap = cm // Use claims for userId/deviceId validation if available
	}


	if userID == "" { // TODO: Enable UserID check in production
		vbl.Stdout.Warn("vid/ws_auth_missing_userid", "Missing x-vibe-user-id header.")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(s.VibeChatErr("vid/ws_auth_missing_userid_err", fmt.Errorf("user identifier missing")))
		return
	}

	if deviceId == "" { // TODO: Enable DeviceID check in production
		vbl.Stdout.Warn("vid/ws_auth_missing_deviceid", "Missing x-vibe-device-id header.")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(s.VibeChatErr("vid/ws_auth_missing_deviceid_err", fmt.Errorf("device identifier missing")))
		return
	}

	// Validate UserID and DeviceID format (e.g., is it a valid ObjectID?)
	userObjectId, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		vbl.Stdout.Warn(vbl.Id("vid/ws_auth_invalid_userid_format"), fmt.Sprintf("Invalid user ID format '%s': %v", userID, err))
		w.WriteHeader(http.StatusBadRequest) // Bad Request for invalid format
		json.NewEncoder(w).Encode(s.VibeChatErr("vid/ws_auth_invalid_userid_format_err", fmt.Errorf("invalid user ID format")))
		return
	}
	if userObjectId.IsZero() {
		vbl.Stdout.Warn(vbl.Id("vid/ws_auth_zero_userid"), fmt.Sprintf("User ID '%s' is zero ObjectID.", userID))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(s.VibeChatErr("vid/ws_auth_zero_userid_err", fmt.Errorf("invalid user ID (zero ID)")))
		return
	}

	deviceObjId, err := primitive.ObjectIDFromHex(deviceId)
	if err != nil {
		vbl.Stdout.Warn(vbl.Id("vid/ws_auth_invalid_deviceid_format"), fmt.Sprintf("Invalid device ID format '%s': %v", deviceId, err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(s.VibeChatErr("vid/ws_auth_invalid_deviceid_format_err", fmt.Errorf("invalid device ID format")))
		return
	}
	if deviceObjId.IsZero() {
		vbl.Stdout.Warn(vbl.Id("vid/ws_auth_zero_deviceid"), fmt.Sprintf("Device ID '%s' is zero ObjectID.", deviceId))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(s.VibeChatErr("vid/ws_auth_zero_deviceid_err", fmt.Errorf("invalid device ID (zero ID)")))
		return
	}


	// TODO: Implement session validation (e.g., checking Redis session based on JWT claims, userId, deviceId)
	// If session validation fails:
	/*
	   if _, err := s.validateSession(jwtToken, userID, deviceId); err != nil { // Need validateSession method
	        vbl.Stdout.Warn(vbl.Id("vid/ws_auth_invalid_session"), fmt.Sprintf("Session validation failed for user %s, device %s: %v", userID, deviceId, err))
	        w.WriteHeader(http.StatusUnauthorized)
	        json.NewEncoder(w).Encode(s.VibeChatErr("vid/ws_auth_invalid_session_err", fmt.Errorf("invalid or expired session")))
	        return
	   }
	*/


	// Get conversation IDs from header or query param
	conversationIdRaw := r.Header.Get("x-vibe-convo-id") // Preferred from header
	if conversationIdRaw == "" {
		conversationIdRaw = r.URL.Query().Get("conversationIds") // Fallback to query param
	}

	if conversationIdRaw == "" {
		vbl.Stdout.Warn(vbl.Id("vid/ws_missing_convid_header"), fmt.Sprintf("Missing conversation id header/query param for user %s, device %s.", userID, deviceId))
		w.WriteHeader(http.StatusBadRequest) // Bad Request if essential parameter is missing
		json.NewEncoder(w).Encode(s.VibeChatErr("c3c43f6e-04d9-4d41-8ebf-d4ef11ebaafb", fmt.Errorf("missing conversation id parameter")))
		return
	}

	var convIds []string
	// Assuming conversationIdRaw is a JSON array string like '["id1", "id2"]'
	if err := json.Unmarshal([]byte(conversationIdRaw), &convIds); err != nil {
		vbl.Stdout.Error(vbl.Id("vid/ws_unmarshal_convids"), fmt.Sprintf("Error unmarshalling conversationIds '%s' for user %s: %v", conversationIdRaw, userID, err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(s.VibeChatErr("6db01ff4-7bcf-49f0-b54a-5a37b0ac9986", fmt.Errorf("invalid conversation IDs format")))
		return
	}

	// Validate conversation IDs format (must be valid ObjectIDs)
	for _, cid := range convIds {
		if _, err := primitive.ObjectIDFromHex(cid); err != nil {
			vbl.Stdout.Warn(vbl.Id("vid/ws_invalid_convid_format"), fmt.Sprintf("Invalid conversation ID format '%s' in list for user %s: %v", cid, userID, err))
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(s.VibeChatErr("vid/ws_invalid_convid_format_err", fmt.Errorf("invalid conversation ID format: %s", cid)))
			return // Stop processing this request
		}
	}

	// --- Upgrade to WebSocket ---
	// Use a WaitGroup if you need to ensure the upgrade happens before this handler returns.
	// However, the standard pattern is that Upgrade takes over the connection and the handler finishes.
	// The logic that runs *after* a successful upgrade should be in a new goroutine launched by Upgrade.
	// Original code used a WaitGroup, which is slightly unusual but works if the goroutine signals completion.
	// Let's stick to the simpler pattern: Upgrade blocks or returns error, on success handle in goroutine.

	// TODO: response headers instead of nil?
	// TODO: re-use upgrader instance? - Yes, upgrader is a global var
	conn, err := upgrader.Upgrade(w, r, nil) // Handshake and upgrade the connection

	// Upgrade failed
	if err != nil {
		// websocket.ErrUpgradeNotSupported is a common error if the client isn't speaking the WS protocol
		if !errors.Is(err, websocket.ErrUpgradeNotSupported) {
			vbl.Stdout.Warn(vbl.Id("vid/ws_upgrade_failed"), fmt.Sprintf("Failed to upgrade connection for user %s, device %s: %v", userID, deviceId, err))
		}
		// Upgrade automatically sends an appropriate HTTP error response (e.g., 426 Upgrade Required)
		// No need to write headers/body here if Upgrade failed.
		return // Handler exits
	}

	// --- Upgrade Successful ---
	// Connection is now a WebSocket connection (`conn`).
	// Handle this connection in a separate goroutine so this ServeHTTP handler can return
	// and the server can accept new connections.

	vbl.Stdout.Info(vbl.Id("vid/ws_upgrade_success"), fmt.Sprintf("Successfully upgraded connection for user %s, device %s. Starting message handler goroutine.", userID, deviceId))


	// Create the UserCtx for this new connection
	// Initialize the UserCtx with the validated IDs and connection details
	clientId := fmt.Sprintf("%s:%s", userID, deviceId) // Recreate clientId
	// Recalculate clientObjId if needed, or rely on user/device IDs
	hash := sha256.Sum256([]byte(clientId))
	clientObjIdHex := hex.EncodeToString(hash[:12]) // First 12 bytes for ObjectID-like hash
	clientObjId, _ := primitive.ObjectIDFromHex(clientObjIdHex) // Should not error if hex is 24 chars

	uc := &vuc.UserCtx{ // Use a pointer to UserCtx
		WSConn: conn,
		UserId:    userID,
		UserObjId: &userObjectId, // Use the validated ObjectID pointer
		DeviceId:    deviceId,
		DeviceObjId: &deviceObjId, // Use the validated ObjectID pointer
		ClientObjId: &clientObjId, // Use the calculated ClientObjectID
		ClientId:    clientId,
		Mtx:     sync.Mutex{}, // Mutex for WS writes
		webrtcPCs: make(map[string]*webrtc.PeerConnection), // Initialize WebRTC PCs map
		webrtcPCsMtx: sync.Mutex{}, // Mutex for WebRTC PCs map
		ConvIds: convIds, // Store the list of conversation IDs for this connection
		Tags: make(map[string]interface{}), // Initialize tags map
		CloseOnce: &sync.Once{}, // Initialize CloseOnce
		OnDisconnects: []func(){}, // Initialize disconnect callbacks
		IsConnClosed: false, // Initially not closed
		// Log: vbl.Stdout.Child(&map[string]interface{}{...}), // Log is removed from UserCtx, use global logger
	}
	// Add user/device/client IDs to logs manually if needed:
	// vbl.Stdout.Child(&map[string]interface{}{"user_id": uc.UserId, "device_id": uc.DeviceId, "client_id": uc.ClientId})


	// Set a close handler on the WebSocket connection. This is called by the gorilla/websocket library
	// when the connection is closed by the peer or due to a network error.
	conn.SetCloseHandler(func(code int, text string) error {
		vbl.Stdout.Info(vbl.Id("vid/ws_close_handler"), fmt.Sprintf("WS Close handler triggered for user %s, device %s. Code: %d, Text: %s", uc.UserId, uc.DeviceId, code, text))
		uc.IsConnClosed = true // Mark the connection as closed
		// The `handleIncomingWSMessages` goroutine will detect this flag (or the read error)
		// and its `defer` block will call `uc.SafeClose` for full cleanup.
		// Returning nil here tells the gorilla/websocket library that we've handled the close.
		return nil
	})

	// Add the new UserCtx pointer to the service's connection maps.
	s.mu.Lock() // Protect Service maps
	// Add to userIdToConns map
	if s.userIdToConns[uc.UserId] == nil {
		s.userIdToConns[uc.UserId] = make(map[*vuc.UserCtx]*vuc.UserCtx)
	}
	s.userIdToConns[uc.UserId][uc] = uc // Use pointer as key and value

	// Add to convIdToConns map for each conversation ID
	for _, convId := range uc.ConvIds {
		if s.convIdToConns[convId] == nil {
			s.convIdToConns[convId] = make(map[*vuc.UserCtx]*vuc.UserCtx)
		}
		s.convIdToConns[convId][uc] = uc // Use pointer as key and value
	}
	s.mu.Unlock() // Release mutex

	// Log current connection counts (optional)
	s.logMemoryStats()


	// Start the goroutine that reads messages from this connection
	// This goroutine will block on ReadMessage until a message or error occurs.
	// The defer in this goroutine handles cleanup (calling uc.SafeClose).
	gor.Gor(func(z gor.Updater) { // Use gor.Gor for tracking/monitoring

		// The deferred function in handleIncomingWSMessages will run when this goroutine exits.
		// It handles the final cleanup including removing the UC from Service maps.
		// We pass the updater 'z' so the message handler can update the goroutine's status.
		s.handleIncomingWSMessages(uc, z)

		// This point is reached only when handleIncomingWSMessages returns (e.g., on read error or close message).
		vbl.Stdout.Info(vbl.Id("vid/ws_handler_goroutine_exit"), fmt.Sprintf("WS message handler goroutine finished for user %s, device %s.", uc.UserId, uc.DeviceId))
	})

	// The ServeHTTP handler finishes here, allowing the main server listener to accept new connections.
	// The newly accepted WebSocket connection is now managed by the handleIncomingWSMessages goroutine.
}

// Removed VibeChatSuccess and VibeChatErr - assuming these are handled by ErrDeets now?
// Looking at original code, VibeChatError was a type, ErrDeets was another.
// Let's keep VibeChatError and VibeChatSuccess if they are distinct or widely used.
// The diff kept them, let's put them back.

//easyjson:json
type VibeChatError struct {
	ErrId      string `json:"errId"`
	ErrMessage string `json:"errMessage"`
}

//easyjson:json
type VibeChatSuccess struct {
	Ok  bool `json:"ok"`
	Msg string `json:"msg"`
	Id  string `json:"id"`
}

func (s *Service) VibeChatSuccess(id string, msg string) *VibeChatSuccess {
	return &VibeChatSuccess{
		Ok:  true,
		Msg: msg,
		Id:  id,
	}
}

func (s *Service) VibeChatErr(id string, err error) *VibeChatError {
	errMsg := "<unknown> error"
	if err != nil {
		errMsg = err.Error()
	}
	vbl.Stdout.Warn(vbl.Id("vid/5f4c17910e19"), "log-id:", id, "error:", errMsg) // Log the error
	return &VibeChatError{
		id, errMsg,
	}
}


// Removed Marshallable interface

// Removed sendChatMessageOut (replace with direct WS relay)

// Removed handleNewChatMessage logic that used brokers (implemented direct WS relay instead)
// The function signature and initial unmarshalling/validation is kept.

// handleNewChatMessage processes a new chat message received from a client over WebSocket.
// It saves the message to MongoDB and relays it to other connected users in the same conversation via their WebSockets.
func (s *Service) handleNewChatMessage(callTrace []string, m *mngo_types.MongoChatMessage, uc *vuc.UserCtx) error {

	var cs = callTrace // Use the provided callTrace

	// This defer is inside the goroutine that calls this function, so it catches panics here.
	// The outer defer in handleIncomingWSMessages catches panics higher up.
	defer func() {
		if r := vhp.HandlePanicWithUserCtxFromWebsocket("66f76469269b", uc); r != nil {
			vbl.Stdout.Error(vbl.Id("vid/f347c68d78a8"), fmt.Sprintf("Panic in handleNewChatMessage: %v", r))
			// Send error back to the original sender
			uc.WriteJSONSafely(cs, s.VibeChatErr(
				"ec080f17-3275-4892-ab68-62d1e2935389",
				fmt.Errorf("unknown error caught by recover()"),
			))
		}
	}()

	// Validate essential fields in the message
	if m.Id.IsZero() {
		errId := "00c11494-d257-4ab0-9c03-087a52f9e912"
		errMessage := fmt.Sprintf("missing message object id: %+v", m)
		vapm.SendTrace(errId, errMessage)
		vbl.Stdout.Error(errId, errMessage)
		uc.WriteJSONSafely(cs, VibeChatError{errId, errMessage})
		return errors.New(errMessage) // Return error
	}
	if m.ChatId.IsZero() {
		errId := "60df47eb-a903-4f70-a0b4-eca527db61ff"
		errMessage := fmt.Sprintf("missing (zero) 'ChatId' in message: %+v", m)
		vbl.Stdout.Error(errId, errMessage)
		vapm.SendTrace(errId, errMessage)
		uc.WriteJSONSafely(cs, VibeChatError{errId, errMessage})
		return errors.New(errMessage) // Return error
	}
	if m.CreatedByUserId.IsZero() {
		errId := "vid/mongo_chat_msg_missing_author"
		errMessage := fmt.Sprintf("missing 'CreatedByUserId' in message: %+v", m)
		vbl.Stdout.Error(errId, errMessage)
		vapm.SendTrace(errId, errMessage)
		uc.WriteJSONSafely(cs, VibeChatError{errId, errMessage})
		return errors.New(errMessage) // Return error
	}
	// Ensure the sender ID matches the user ID associated with this connection
	if uc.UserObjId == nil || v.CreatedByUserId.Hex() != uc.UserObjId.Hex() {
		errId := "vid/mongo_chat_msg_author_mismatch"
		errMessage := fmt.Sprintf("message author mismatch: expected %s, got %s", uc.UserObjId.Hex(), v.CreatedByUserId.Hex())
		vbl.Stdout.Warn(errId, errMessage)
		// Decide how to handle this security/logic issue. Deny the message?
		uc.WriteJSONSafely(cs, VibeChatError{errId, errMessage})
		return errors.New(errMessage) // Return error
	}

	vbl.Stdout.Info(vbl.Id("vid/d2a975309b7b"), fmt.Sprintf("Handling new chat message %s from user %s in conv %s", m.Id.Hex(), m.CreatedByUserId.Hex(), m.ChatId.Hex()))


	// --- Message Handling Logic ---
	// 1. Save to MongoDB
	// 2. Relay to other connected clients in the same conversation over WS

	// Save to MongoDB
	go func() { // Run DB operation concurrently
		defer func() { // Recover from panics
			if r := recover(); r != nil {
				vbl.Stdout.Error(vbl.Id("vid/mongo_insert_msg_panic"), fmt.Sprintf("Panic during Mongo insert for message %s: %v", m.Id.Hex(), r))
				vapm.SendTrace("vid/mongo_insert_msg_panic_apm", fmt.Sprintf("%v", r))
				// No need to send error back here
			}
		}()
		vbl.Stdout.Info(vbl.Id("vid/mongo_insert_msg"), fmt.Sprintf("Attempting to save message '%s' to mongo for conv '%s'", m.Id.Hex(), m.ChatId.Hex()))
		coll := s.M.Col.ChatConvMessages // Assuming collection is available
		if _, err := s.M.DoInsertOne(vctx.NewStdMgoCtx(), coll, m); err != nil {
			vapm.SendTrace("4c4a70d4-6fef-42ab-9747-1510cd789955", fmt.Sprintf("Mongo insert error for message %s: %v", m.Id.Hex(), err))
			vbl.Stdout.Error(vbl.Id("vid/26cd003aa2e9"), fmt.Sprintf("Error inserting chat message %s into Mongo: %v", m.Id.Hex(), err))
			// Decide if you send an error back to the client if DB save fails
			// uc.WriteJSONSafely(cs, s.VibeChatErr("mongo_insert_msg_err", fmt.Errorf("failed to save message")))
		} else {
			vbl.Stdout.Debug(vbl.Id("vid/mongo_insert_msg_success"), fmt.Sprintf("Successfully wrote new chat message %s to mongo", m.Id.Hex()))
			// Send an acknowledgement back to the *sender* client once saved to DB
			ackPayload := map[string]interface{}{
				"ack": true,
				"reason": "mongo received",
				"convId": m.ChatId.Hex(),
				"messageId": m.Id.Hex(),
			}
			outgoingAck := WebsocketOutgoingPayload{
				Meta: WSOutgoingMeta{FromSource: "Server", TimeCreated: time.Now().UTC().String()},
				Messages: []interface{}{map[string]interface{}{"@vibe-type": "message-ack", "@vibe-data": ackPayload}}, // Example structure
			}
			if ackErr := uc.WriteJSONSafely(cs, outgoingAck); ackErr != nil {
				vbl.Stdout.Warn(vbl.Id("vid/ws_send_ack_err"), fmt.Sprintf("Error sending message ack for %s to sender %s: %v", m.Id.Hex(), uc.UserId, ackErr))
			} else {
				vbl.Stdout.Debug(vbl.Id("vid/ws_send_ack_success"), fmt.Sprintf("Sent message ack for %s to sender %s", m.Id.Hex(), uc.UserId))
			}
		}
	}()


	// Relay message to other connected clients in the same conversation over WS
	go func() { // Run relaying concurrently
		defer func() { // Recover from panic during relay
			if r := recover(); r != nil {
				vbl.Stdout.Error(vbl.Id("vid/ws_relay_msg_panic"), fmt.Sprintf("%v", r))
				vapm.SendTrace("vid/ws_relay_msg_panic_apm", fmt.Sprintf("%v", r))
				// No need to send error back here
			}
		}()

		convIdStr := m.ChatId.Hex()
		s.mu.Lock() // Protect the convIdToConns map while reading it
		connsInConv := s.convIdToConns[convIdStr] // Get map of connections for this conversation
		s.mu.Unlock() // Release lock quickly

		if connsInConv != nil && len(connsInConv) > 0 {
			vbl.Stdout.Debug(vbl.Id("vid/ws_relay_msg_start"), fmt.Sprintf("Relaying message %s to %d connections in conversation %s", m.Id.Hex(), len(connsInConv), convIdStr))

			// Prepare the message payload to relay. It should include the message data.
			// Use a structure the client's WS handler expects for new chat messages.
			// Example: map with type and data
			relayMsgPayload := map[string]interface{}{"@vibe-type": "new-chat-message", "@vibe-data": m} // Pass the MongoChatMessage struct 'm'

			// Wrap in the standard outgoing payload structure
			outgoingRelayPayload := WebsocketOutgoingPayload{
				Meta: WSOutgoingMeta{FromSource: "Server-Relay", TimeCreated: time.Now().UTC().String()},
				Messages: []interface{}{relayMsgPayload},
			}

			// Iterate over connections in the conversation and send the message
			// Iterate over a copy of the map keys or use a goroutine per send to avoid blocking
			connKeys := make([]*vuc.UserCtx, 0, len(connsInConv))
			for k := range connsInConv {
				connKeys = append(connKeys, k)
			}

			for _, otherUc := range connKeys {
				// Do not send the message back to the original sender's connection
				if otherUc != uc { // Compare pointers
					go func(targetUc *vuc.UserCtx) { // Send concurrently to each recipient
						defer func() { // Recover from panics within recipient goroutine
							if r := recover(); r != nil {
								vbl.Stdout.Error(vbl.Id("vid/ws_relay_recipient_panic"), fmt.Sprintf("Panic sending relayed message %s to user %s conn in conv %s: %v", m.Id.Hex(), targetUc.UserId, convIdStr, r))
							}
						}()
						vbl.Stdout.Debug(vbl.Id("vid/ws_relay_sending"), fmt.Sprintf("Sending relayed message %s to user %s connection in conv %s.", m.Id.Hex(), targetUc.UserId, convIdStr))
						if err := targetUc.WriteJSONSafely(cs, outgoingRelayPayload); err != nil {
							// Log error if sending fails for a specific peer (e.g., they disconnected).
							// The cleanup of disconnected peers is handled elsewhere (defer in handleIncomingWSMessages).
							vbl.Stdout.Warn(vbl.Id("vid/ws_relay_write_err"), fmt.Sprintf("Error sending relayed message %s to user %s connection in conv %s: %v", m.Id.Hex(), targetUc.UserId, convIdStr, err))
						}
					}(otherUc) // Pass the UserCtx pointer
				}
			}
			vbl.Stdout.Debug(vbl.Id("vid/ws_relay_msg_done"), fmt.Sprintf("Finished dispatching relayed message %s for conversation %s.", m.Id.Hex(), convIdStr))

		} else {
			vbl.Stdout.Debug(vbl.Id("vid/ws_relay_msg_no_conns"), fmt.Sprintf("No other connections found in conversation %s to relay message %s.", convIdStr, m.Id.Hex()))
		}
	}()


	// Return nil if initial unmarshalling and basic checks passed.
	// Errors during DB save or relaying are handled and logged within their goroutines.
	// This function itself does not need to return an error unless the initial processing fails.
	return nil
}


// Removed consumeMultipleTopicsFromRedisChannel and consumeMultipleTopicsFromRabbitMQ


// getAllMessagesForParticularChats fetches and sends messages for specific conversations from MongoDB.
// It sends messages individually over the WebSocket connection.
func (s *Service) getAllMessagesForParticularChats(uc *vuc.UserCtx, convObjIds []primitive.ObjectID) error {
	var userObjectId = uc.UserObjId // Get the user's ObjectID from the UserCtx

	if userObjectId == nil || userObjectId.IsZero() {
		// This should ideally not happen if connection setup validated the user ID, but check defensively.
		err := vbu.ErrorFromArgs("c03a9d41-0ff0-4c6f-b777-42ef4ddf94e0", "user object ID is nil or zero")
		vbl.Stdout.Error(err)
		uc.WriteJSONSafely(vbu.GetFilteredStacktrace(), s.VibeChatErr("c03a9d41-0ff0-4c6f-b777-42ef4ddf94e0", err))
		return err
	}

	var cs = vbu.GetFilteredStacktrace() // Get call stack for logging/error reporting

	if len(convObjIds) < 1 {
		vbl.Stdout.Warn(
			vbl.Id("vid/4c428717b030"),
			fmt.Sprintf("Called getAllMessagesForParticularChats for user %s with no conversation IDs.", uc.UserId),
		)
		// Send an error back or just acknowledge success with no messages
		uc.WriteJSONSafely(cs, s.VibeChatErr(
			"9b5db08f-1afa-4bd8-a615-f47b2d64ac4d",
			fmt.Errorf("Missing conversation IDs in polling request"),
		))
		return vbu.ErrorFromArgs("6714af34-de2f-4f8f-9f9f-a6beb16e2313", "missing conversation ids for polling")
	}

	vbl.Stdout.Info(vbl.Id("vid/mongo_poll_convs"), fmt.Sprintf("Polling MongoDB for messages for user %s in conversations: %v", uc.UserId, convObjIds))

	// This defer catches panics within this specific goroutine.
	defer func() {
		if r := recover(); r != nil {
			vbl.Stdout.Error(vbl.Id("vid/mongo_poll_convs_panic"), fmt.Sprintf("%v", r))
			uc.WriteJSONSafely(cs, s.VibeChatErr(
				"698bbdc1-3d5c-442a-a0bf-85762939284a",
				fmt.Errorf("Panic/Exception during message polling"),
			))
		}
	}()

	// TODO: Implement fetching based on timestamps or message IDs the client already has
	// The current query fetches all messages for these conv IDs, excluding those created by the requesting user (optional).

	filter := bson.M{
		"$or": []interface{}{ // Find messages where the ChatId is in the list OR the user is a PriorityUser (though ChatConvUsers lookup is better)
			// Filter by ChatId directly is often sufficient if client provides all relevant IDs.
			// Using $in on ChatId is more efficient for querying by conversation.
			bson.M{
				"ChatId": bson.M{
					"$in": convObjIds, // Messages belonging to these conversations
				},
			},
			// Including the PriorityUserIds check here might be redundant or incorrect
			// if the client explicitly asks for messages by conversation ID.
			// If a user is in a conversation, the client should provide that conversation's ID.
			// Keep it for now if the schema/logic relies on it.
			bson.M{
				"PriorityUserIds": userObjectId, // Messages specifically marked for this user (less common for general chat)
			},
		},
		// Optionally exclude messages the user has already received or acknowledged
		// This requires additional state tracking (e.g., last received message ID per conversation per device)
		// "Id": bson.M{"$nin": clientKnownMessageIds}, // Need clientKnownMessageIds from request

		// Optionally exclude messages created by the user themselves, if client doesn't need to refetch them.
		// "CreatedByUserId": bson.M{"$ne": userObjectId}, // Check if client needs their own messages on poll
	}

	// Sorting options (e.g., by creation time)
	findOptions := options.Find().SetSort(bson.D{{"CreatedAt", 1}}) // Sort by creation time ascending

	// Using a projection can reduce data transfer if only certain fields are needed initially.
	// findOptions.SetProjection(bson.D{...})


	// Use CursorCallbackByRow to process messages one by one as they are retrieved,
	// sending them over the WebSocket without buffering all results in memory.
	s.M.CursorCallbackByRow(
		vctx.NewStdMgoCtx(), // Standard MongoDB context
		s.M.Col.ChatConvMessages, // Target collection
		filter, // Query filter
		findOptions, // Find options (sorting, projection)
		func(doc *bson.M) { // Callback function for each document (message) found
			// This callback runs for each message returned by the query.

			// Check if the connection is still open before attempting to send
			if uc.IsConnClosed {
				vbl.Stdout.Debug(vbl.Id("vid/mongo_poll_conn_closed"), fmt.Sprintf("Connection closed for user %s during message polling. Stopping send.", uc.UserId))
				// Return false from the callback to stop cursor iteration if supported by CursorCallbackByRow
				// (Assuming CursorCallbackByRow checks the return value and stops if false)
				// If not, need to add a channel mechanism to signal this goroutine to stop.
				return // Exit the callback, hoping CursorCallbackByRow respects it
			}


			vbl.Stdout.Debug(vbl.Id("vid/mongo_poll_send"), fmt.Sprintf("Sending polled message to user %s: %+v", uc.UserId, *doc))

			// Prepare the message to send over WebSocket
			// Using a specific type/structure the client expects for polled messages.
			var messageToClient = struct {
				MessageType   string      `json:"messageType"`
				MessageFrom   string      `json:"messageFrom"` // Source indicator
				UnreadMessage interface{} `json:"unreadMessage"` // The message document
			}{
				MessageType:   "vibe_polled_message", // Client-side type identifier
				MessageFrom:   "mongo_poll_convs",
				UnreadMessage: doc, // Send the raw document map
			}

			// Wrap in the standard outgoing payload and send safely
			outgoingPayload := WebsocketOutgoingPayload{
				Meta: WSOutgoingMeta{FromSource: "Mongo-Poll-Convs", TimeCreated: time.Now().UTC().String()},
				Messages: []interface{}{messageToClient},
			}

			if err := uc.WriteJSONSafely(cs, outgoingPayload); err != nil {
				// If writing fails (e.g., connection closed mid-poll), log the error.
				// The main read loop handler will eventually detect the closed connection and clean up.
				vbl.Stdout.Warn(vbl.Id("vid/mongo_poll_write_err"), fmt.Sprintf("Error sending polled message to user %s: %v", uc.UserId, err))
				// Decide if this write error should stop the polling for this connection.
				// Returning false from the callback might signal CursorCallbackByRow to stop.
				// return false // Assuming CursorCallbackByRow checks this
			}
			// return true // Continue processing next document
		},
		// TODO: Add error handler func to CursorCallbackByRow if it has one
		// onError: func(err error) { ... }
	)

	// Optional: Send a completion message to the client after polling finishes
	if !uc.IsConnClosed { // Only send if connection is still open
		completionMsg := map[string]interface{}{
			"messageType": "vibe_poll_completion",
			"status": "done",
			"conversationsPolled": len(convObjIds),
		}
		outgoingCompletion := WebsocketOutgoingPayload{
			Meta: WSOutgoingMeta{FromSource: "Mongo-Poll-Convs", TimeCreated: time.Now().UTC().String()},
			Messages: []interface{}{completionMsg},
		}
		if err := uc.WriteJSONSafely(cs, outgoingCompletion); err != nil {
			vbl.Stdout.Warn(vbl.Id("vid/mongo_poll_completion_err"), fmt.Sprintf("Error sending poll completion to user %s: %v", uc.UserId, err))
		} else {
			vbl.Stdout.Info(vbl.Id("vid/mongo_poll_completion_sent"), fmt.Sprintf("Sent poll completion to user %s.", uc.UserId))
		}
	}


	return nil
}


// getMongoMessagesForAllChatsByUser fetches and sends all messages for all chats involving a specific user from MongoDB.
// Similar to getAllMessagesForParticularChats, but discovers chats first.
func (s *Service) getMongoMessagesForAllChatsByUser(callTrace []string, uc *vuc.UserCtx) error {
	var userObjectId = uc.UserObjId // Get the user's ObjectID

	if userObjectId == nil || userObjectId.IsZero() {
		err := vbu.ErrorFromArgs("388fc07f-865d-4df6-bccc-391bdff16e11", "user object ID is nil or zero")
		vbl.Stdout.Error(err)
		uc.WriteJSONSafely(vbu.GetFilteredStacktrace(), s.VibeChatErr("388fc07f-865d-4df6-bccc-391bdff16e11", err))
		return err
	}

	var userID = uc.UserId // String ID
	var cs = callTrace // Use provided callTrace
	vbl.Stdout.Info(vbl.Id("vid/mongo_poll_all"), fmt.Sprintf("Polling MongoDB for ALL messages for user %s.", userID))


	// This defer catches panics within this specific goroutine.
	defer func() {
		if r := recover(); r != nil {
			vbl.Stdout.Error(vbl.Id("vid/mongo_poll_all_panic"), fmt.Sprintf("%v", r))
			uc.WriteJSONSafely(cs, s.VibeChatErr(
				"d2ecd2ae-95d4-4261-a9a7-68a761135782",
				fmt.Errorf("Panic/Exception during all message polling"),
			))
		}
	}()

	// Step 1: Find all conversation IDs that the user is a part of.
	// Using a limiter channel to control concurrent fetching for each conversation found.
	var limiter = make(chan struct{}, 5) // Allow up to 5 concurrent chat message fetches

	listedOfMutedChatIds := []primitive.ObjectID{} // TODO: Implement fetching muted chat IDs

	filterChats := bson.M{
		"$and": []interface{}{
			bson.M{"UserId": userObjectId}, // Find entries in ChatConvUsers where this user is listed
			bson.M{"ChatId": bson.M{
				"$nin": listedOfMutedChatIds, // Exclude muted chats
			}},
		},
	}

	// Project only the ChatId field to minimize data transferred
	findChatOptions := options.Find().SetProjection(bson.D{{"ChatId", 1}})

	s.M.CursorCallbackByRow(
		vctx.NewStdMgoCtx(),
		s.M.Col.ChatConvUsers, // Query the collection linking users to conversations
		filterChats,
		findChatOptions,
		func(chatUserDoc *bson.M) { // Callback for each conversation found
			defer func() { // Release a slot in the limiter when this callback finishes
				<-limiter
			}()
			limiter <- struct{}{} // Acquire a slot in the limiter (blocks if full)

			// Check if the connection is still open before proceeding
			if uc.IsConnClosed {
				vbl.Stdout.Debug(vbl.Id("vid/mongo_poll_all_conn_closed_chat"), fmt.Sprintf("Connection closed for user %s during chat discovery. Stopping.", uc.UserId))
				return // Stop processing chats
			}


			vbl.Stdout.Debug(vbl.Id("vid/mongo_poll_all_chat_found"), fmt.Sprintf("Found chat association for user %s: %+v", uc.UserId, *chatUserDoc))

			chatId, ok := (*chatUserDoc)["ChatId"]
			if !ok {
				vbl.Stdout.Error(vbl.Id("vid/mongo_poll_all_missing_chatid"), fmt.Sprintf("Missing 'ChatId' field in ChatConvUsers document for user %s: %+v", uc.UserId, *chatUserDoc))
				// Send an error back or just log? Log for now.
				// uc.WriteJSONSafely(cs, s.VibeChatErr("4cf08c98-971f-45d0-a81f-265c2b4eb554", fmt.Errorf("chatid not in result")))
				return // Skip this chat entry
			}

			chatObjectId, err := vbu.GetObjectId(chatId) // Convert to ObjectID
			if err != nil || chatObjectId == nil || chatObjectId.IsZero() {
				vbl.Stdout.Error(vbl.Id("vid/mongo_poll_all_invalid_chatid"), fmt.Sprintf("Invalid ChatId '%v' from ChatConvUsers for user %s: %v", chatId, uc.UserId, err))
				// uc.WriteJSONSafely(cs, s.VibeChatErr("adc5f004-1d7c-4261-93bb-f7bba1b10a60", fmt.Errorf("invalid chat id from DB")))
				return // Skip this chat
			}

			// Step 2: For each conversation found, fetch its messages.
			// This should run concurrently for different conversations, controlled by the limiter.
			go func(convObjId primitive.ObjectID) { // Goroutine to fetch messages for a single conversation
				defer func() { // Recover from panic in this inner goroutine
					if r := recover(); r != nil {
						vbl.Stdout.Error(vbl.Id("vid/mongo_poll_chat_msgs_panic"), fmt.Sprintf("Panic during message fetch for conv %s (user %s): %v", convObjId.Hex(), uc.UserId, r))
						vapm.SendTrace("vid/mongo_poll_chat_msgs_panic_apm", fmt.Sprintf("%v", r))
						// No need to send error back here, main handler does
					}
				}()

				vbl.Stdout.Debug(vbl.Id("vid/mongo_poll_chat_msgs"), fmt.Sprintf("Fetching messages for conversation %s for user %s.", convObjId.Hex(), uc.UserId))

				// Define the filter for messages in this specific conversation
				filterMessages := bson.M{
					"ChatId": convObjId,
					// TODO: Implement fetching based on timestamps or message IDs the client already has
					// "Id": bson.M{"$nin": clientKnownMessageIdsForThisConv}, // Need client state per conv
					// Optionally exclude messages created by the user themselves, if client doesn't need to refetch them.
					// "CreatedByUserId": bson.M{"$ne": userObjectId}, // Check if client needs their own messages on poll
				}
				// Sort messages by creation time
				findMessageOptions := options.Find().SetSort(bson.D{{"CreatedAt", 1}}) // Sort by creation time ascending


				// Use CursorCallbackByRow to process messages for this conversation
				s.M.CursorCallbackByRow(
					vctx.NewStdMgoCtx(),
					s.M.Col.ChatConvMessages, // Query the messages collection
					filterMessages,
					findMessageOptions,
					func(msgDoc *bson.M) { // Callback for each message found in this conversation
						// This callback runs for each message.

						// Check if the connection is still open
						if uc.IsConnClosed {
							vbl.Stdout.Debug(vbl.Id("vid/mongo_poll_msgs_conn_closed"), fmt.Sprintf("Connection closed for user %s during message fetch for conv %s. Stopping send.", uc.UserId, convObjId.Hex()))
							// Return false from callback to signal stop (if supported)
							return // Exit callback
						}

						vbl.Stdout.Trace(vbl.Id("vid/mongo_poll_msg_send"), fmt.Sprintf("Sending polled message to user %s (conv %s): %+v", uc.UserId, convObjId.Hex(), *msgDoc))

						// Prepare and send the message over WebSocket
						var messageToClient = struct {
							MessageType   string      `json:"messageType"`
							MessageFrom   string      `json:"messageFrom"`
							UnreadMessage interface{} `json:"unreadMessage"`
						}{
							MessageType:   "vibe_polled_message",
							MessageFrom:   "mongo_poll_all_chats", // Source indicator
							UnreadMessage: msgDoc, // Send the raw document map
						}

						// Wrap in the standard outgoing payload and send safely
						outgoingPayload := WebsocketOutgoingPayload{
							Meta: WSOutgoingMeta{FromSource: "Mongo-Poll-All", TimeCreated: time.Now().UTC().String()},
							Messages: []interface{}{messageToClient},
						}

						if err := uc.WriteJSONSafely(cs, outgoingPayload); err != nil {
							// If writing fails (e.g., connection closed mid-poll), log the error.
							vbl.Stdout.Warn(vbl.Id("vid/mongo_poll_msg_write_err"), fmt.Sprintf("Error sending polled message from conv %s to user %s: %v", convObjId.Hex(), uc.UserId, err))
							// Decide if this write error should stop the polling for this specific conversation.
							// Returning false might signal CursorCallbackByRow to stop.
							// return false // Assuming CursorCallbackByRow checks this
						}
						// return true // Continue processing next message in this conversation
					},
					// TODO: Add error handler func to CursorCallbackByRow
				) // End CursorCallbackByRow for messages

				vbl.Stdout.Debug(vbl.Id("vid/mongo_poll_chat_msgs_done"), fmt.Sprintf("Finished fetching messages for conversation %s for user %s.", convObjId.Hex(), uc.UserId))

			}(*chatObjectId) // Pass the conversation ObjectID to the goroutine

		}, // End Callback for each conversation found
		// TODO: Add error handler func to CursorCallbackByRow for finding chats
	) // End CursorCallbackByRow for finding chats

	// Wait for all message fetching goroutines to finish (those started by the inner loop's callback)
	// Fill the limiter channel completely to ensure all started goroutines have tried to acquire a slot and finished.
	for i := 0; i < cap(limiter); i++ {
		limiter <- struct{}{}
	}
	// At this point, all message fetching goroutines started by the chat discovery have completed or exited early.

	// Optional: Send a completion message to the client after polling all chats finishes
	if !uc.IsConnClosed { // Only send if connection is still open
		completionMsg := map[string]interface{}{
			"messageType": "vibe_poll_completion",
			"status": "done_all",
			// Could include count of chats/messages polled
		}
		outgoingCompletion := WebsocketOutgoingPayload{
			Meta: WSOutgoingMeta{FromSource: "Mongo-Poll-All", TimeCreated: time.Now().UTC().String()},
			Messages: []interface{}{completionMsg},
		}
		if err := uc.WriteJSONSafely(cs, outgoingCompletion); err != nil {
			vbl.Stdout.Warn(vbl.Id("vid/mongo_poll_all_completion_err"), fmt.Sprintf("Error sending poll all completion to user %s: %v", uc.UserId, err))
		} else {
			vbl.Stdout.Info(vbl.Id("vid/mongo_poll_all_completion_sent"), fmt.Sprintf("Sent poll all completion to user %s.", uc.UserId))
		}
	}

	return nil
}


