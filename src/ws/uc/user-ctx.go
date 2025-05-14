package vuc

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/oresoftware/chat.webrtc/src/common/vibelog" // Assuming this is needed for logging
	vbl "github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3" // Add Pion import
)

// UserCtx holds context information for a single WebSocket connection and associated resources (like WebRTC PeerConnections).
type UserCtx struct {
	// Connection state
	CloseOnce     *sync.Once // Ensures SafeClose is called only once
	OnDisconnects []func()   // Callbacks to run on disconnect
	IsConnClosed  bool       // Flag indicating if the connection is closed

	// Client identification
	ClientObjId *primitive.ObjectID // Unique ID for the client (user+device)
	ClientId    string              // String representation of ClientId
	UserObjId   *primitive.ObjectID // MongoDB ObjectID for the user
	UserId      string              // String representation of User ID
	DeviceObjId *primitive.ObjectID // MongoDB ObjectID for the device
	DeviceId    string              // String representation of Device ID

	// WebRTC related fields
	webrtcPCs    map[string]*webrtc.PeerConnection // map[callID] -> PeerConnection
	webrtcPCsMtx sync.Mutex                        // Mutex to protect the webrtcPCs map

	// WebSocket connection
	WSConn *websocket.Conn // The active WebSocket connection
	Mtx    sync.Mutex      // Mutex to protect writes to the WebSocket connection

	// Conversation/Topic Subscriptions
	ConvIds []string               // List of conversation IDs the user is subscribed to (via this connection)
	Tags    map[string]interface{} // Arbitrary tags

	// Note: Log field from jlog is removed, using global vibelog directly or passing logger instances
	// Note: RabbitMQ/Kafka/Redis fields are removed
	// RMQChannel, RMQMtx, publishLock, rmqChannelLastUsed, etc. removed
}

// FireOnDisconnect executes all registered disconnect callbacks.
func (uc *UserCtx) FireOnDisconnect() {
	for _, f := range uc.OnDisconnects {
		f()
	}
}

// OnDisconnect registers a function to be called when the connection is closed.
func (uc *UserCtx) OnDisconnect(f func()) {
	uc.OnDisconnects = append(uc.OnDisconnects, f)
}

// WriteMessageSafely sends a WebSocket message securely, preventing concurrent writes.
func (uc *UserCtx) WriteMessageSafely(messageType int, data []byte) error {
	uc.Mtx.Lock()
	defer uc.Mtx.Unlock()

	if uc.WSConn == nil {
		// Connection already nil or closed
		return errors.New("websocket connection is nil or already closed")
	}

	if err := uc.WSConn.WriteMessage(messageType, data); err != nil {
		// Log the error, SafeClose will handle cleanup if needed
		// uc.Log.Warn(vbl.Id("vid/9d157baaa874"), err) // Removed uc.Log
		vbl.Stdout.Warn(vbl.Id("vid/9d157baaa874"), fmt.Sprintf("Error writing WS message: %v", err))
		return err
	}

	return nil
}

// SafeClose attempts to gracefully close the WebSocket connection and associated resources (like WebRTC PeerConnections).
func (uc *UserCtx) SafeClose(ignoreError bool) error {
	// Need to lock to prevent concurrent closes and writes
	uc.Mtx.Lock()
	defer uc.Mtx.Unlock()

	// Ensure this block runs only once
	uc.CloseOnce.Do(func() {
		vbl.Stdout.Info(vbl.Id("vid/uc_safe_close"), fmt.Sprintf("Closing connection for user %s, device %s", uc.UserId, uc.DeviceId))

		conn := uc.WSConn // Get a local ref to obj

		if conn == nil {
			vbl.Stdout.Error(vbl.Id("vid/12cd8a4741d1"), "missing conn (nil) during SafeClose")
			// Already closed or never initialized properly
			uc.IsConnClosed = true // Ensure the flag is set
			uc.FireOnDisconnect()  // Fire callbacks even if conn was nil
			return                 // Nothing more to close for this connection
		}

		// Close WebRTC PeerConnections first
		uc.webrtcPCsMtx.Lock()
		for callID, pc := range uc.webrtcPCs {
			vbl.Stdout.Info(vbl.Id("vid/webrtc_pc_close"), fmt.Sprintf("Closing WebRTC PeerConnection for call %s (user %s)", callID, uc.UserId))
			if closeErr := pc.Close(); closeErr != nil && !errors.Is(closeErr, webrtc.ErrConnectionClosed) {
				vbl.Stdout.Warn(vbl.Id("vid/webrtc_pc_close_err"), fmt.Sprintf("Error closing WebRTC PC for call %s (user %s): %v", callID, closeErr))
			}
			delete(uc.webrtcPCs, callID) // Remove from map immediately
		}
		uc.webrtcPCsMtx.Unlock()

		// Send a close message to the client for a graceful shutdown
		closeMessage := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		if err := conn.WriteMessage(websocket.CloseMessage, closeMessage); err != nil {
			if ignoreError {
				vbl.Stdout.Debug(vbl.Id("vid/167a70968cb7"), fmt.Sprintf("Error writing WS close message (ignored): %v", err))
			} else {
				vbl.Stdout.Warn(vbl.Id("vid/75d565e715b5"), fmt.Sprintf("Error writing WS close message: %v", err))
			}
		}

		// Close the underlying WebSocket connection
		if err := conn.Close(); err != nil {
			if ignoreError {
				vbl.Stdout.Debug(vbl.Id("vid/dd3ddc35e286"), fmt.Sprintf("Error closing WS connection (ignored): %v", err))
			} else {
				vbl.Stdout.Warn(vbl.Id("vid/379183697c57"), fmt.Sprintf("Error closing WS connection: %v", err))
			}
		}

		// Mark the connection as closed after attempting close operations
		uc.IsConnClosed = true

		// Fire callbacks for disconnect event
		uc.FireOnDisconnect()
	})

	// Return an error if SafeClose failed during the initial call (subsequent calls are no-ops)
	// Check the IsConnClosed flag set within the Do func if needed, or just return nil if it was called
	return nil // SafeClose is designed to be idempotent and cleanup best effort
}

// jsonMarshaler is an interface for types that can marshal themselves to JSON.
type jsonMarshaler interface {
	MarshalJSON() ([]byte, error)
}

// WSReplyMeta holds metadata for WebSocket replies.
// easyjson:json
type WSReplyMeta struct {
	StackTrace []string `json:"stackTrace,omitempty"`
}

// WSReply is the standard structure for messages sent from the server over WebSocket.
// easyjson:json
type WSReply struct {
	Meta WSReplyMeta `json:"meta"`
	Data interface{} `json:"data"`
}

// WriteJSONSafely sends a JSON message over the WebSocket securely.
func (uc *UserCtx) WriteJSONSafely(callTrace []string, v interface{}) error {
	uc.Mtx.Lock()
	defer uc.Mtx.Unlock()

	if uc.WSConn == nil || uc.IsConnClosed {
		// Attempting to write to a closed or nil connection
		return errors.New("attempting to write to closed or nil websocket connection")
	}

	var cs []string
	// Only get stack trace if configured and not in production for performance/verbosity
	// Assuming config is accessible or passed down. Let's get it directly for now.
	cfg := vbl.GetLogConf() // Assuming vibelog config includes this setting
	if cfg.EnableStackTraces {
		// Ensure GetNewStacktraceFrom is available or implement stack trace capture
		// cs = vbu.GetNewStacktraceFrom(callTrace) // Need vbu import, GetNewStacktraceFrom func
		// For simplicity in this refactor, let's just use the incoming callTrace or a simple capture
		// Assuming callTrace is already formatted as needed or can be logged separately.
		// If stack trace is expensive, only capture it on error replies.
		cs = callTrace // Use the provided callTrace

	}

	// Wrap the data in the standard WSReply structure
	var wsr = WSReply{
		Meta: WSReplyMeta{StackTrace: cs},
		Data: v,
	}

	// Marshal the WSReply structure to JSON bytes
	// Using json.Marshal as easyjson marshalling isn't automatically attached here
	// or requires generating marshalers for WSReply/WSReplyMeta and potentially 'v'.
	// If 'v' is a complex struct that needs easyjson, you'd need to ensure it has MarshalJSON.
	// For simplicity and handling interface{}, standard json.Marshal is more robust here.
	jsonData, err := json.Marshal(wsr)
	if err != nil {
		// This is an error marshalling the *outgoing* message. Log it and return.
		// Do not attempt to write to the WS conn as the data is invalid.
		vbl.Stdout.Error(vbl.Id("vid/ws_write_json_marshal_err"), fmt.Sprintf("Failed to marshal outgoing WS message: %v, data: %+v", err, v))
		return err
	}

	if err := uc.WSConn.WriteMessage(websocket.TextMessage, jsonData); err != nil {
		// Error writing to the connection (e.g., connection closed)
		vbl.Stdout.Warn(vbl.Id("vid/f1e0fb06755b"), fmt.Sprintf("Error writing JSON to WS connection: %v", err))
		return err
	}

	return nil
}

// SendWebRTCMessage sends a structured WebRTC signaling message over the WebSocket.
// This function should be called by WebRTC event handlers (like OnICECandidate, OnNegotiationNeeded)
// or other parts of the server logic that need to signal the client.
func (uc *UserCtx) SendWebRTCMessage(callTrace []string, msgType string, payload interface{}) error {
	// This wraps the WebRTC message payload in the existing WebSocket message format
	// for signaling, using the "@vibe-type": "webrtc-signaling" structure expected
	// by the client-side WS handler.

	// easyjson:json
	// This struct is the content of the "@vibe-data" field
	type WebRTCSignalingPayloadData struct {
		Type    string      `json:"type"`    // "offer", "answer", "ice", "data", "close", "status", "ice_status", "signaling_status", "datachannel_status"
		CallId  string      `json:"callId"`  // Call identifier
		Payload interface{} `json:"payload"` // The SDP, ICE candidate, or DataChannel message payload
	}

	// The payload passed to this function *must* contain the "callId" field
	// so we know which call the message belongs to.
	payloadMap, ok := payload.(map[string]interface{})
	if !ok {
		vbl.Stdout.Error(vbl.Id("vid/webrtc_send_invalid_payload"), fmt.Sprintf("Invalid payload type for SendWebRTCMessage (expected map[string]interface{}): %+v", payload))
		return errors.New("invalid payload type for webrtc signaling")
	}

	callIdVal, ok := payloadMap["callId"]
	if !ok {
		vbl.Stdout.Error(vbl.Id("vid/webrtc_send_missing_callid"), fmt.Sprintf("Payload for SendWebRTCMessage missing 'callId': %+v", payload))
		return errors.New("payload missing callId")
	}
	callId, ok := callIdVal.(string)
	if !ok {
		vbl.Stdout.Error(vbl.Id("vid/webrtc_send_invalid_callid"), fmt.Sprintf("Payload 'callId' is not a string: %+v", payload))
		return errors.New("payload 'callId' is not a string")
	}

	signalingData := WebRTCSignalingPayloadData{
		Type:    msgType,
		CallId:  callId,
		Payload: payload, // The original payload is nested here
	}

	// Wrap this into the existing outgoing structure format expected by the client's
	// handleIncomingWSMessages counterpart (assuming the client also expects the List structure)
	// Or maybe the client's WS handler just expects the '@vibe-type'/`@vibe-data` structure directly?
	// Let's stick to the structure seen in handleIncomingWSMessages for consistency: `List []map[string]interface{}`
	outgoingMsg := map[string]interface{}{ // Represents one item in the 'List'
		"@vibe-type": "webrtc-signaling",
		"@vibe-data": signalingData, // The struct containing type, callId, and payload
		// Add any necessary @vibe-meta here if needed for signaling, e.g., sequence numbers
		// "@vibe-meta": map[string]interface{}{ ... },
	}

	// The final message sent over the WebSocket will be a `WebsocketOutgoingPayload`
	// with this map inside its `Messages` slice.
	// This means we need to call uc.WriteJSONSafely with WebsocketOutgoingPayload.
	// Let's adapt uc.WriteJSONSafely to correctly handle this.

	// Create the full outgoing payload
	fullOutgoingPayload := WebsocketOutgoingPayload{
		Meta: WSOutgoingMeta{
			FromSource:  "WebRTC-Signaling-Server",
			TimeCreated: time.Now().UTC().String(),
		},
		Messages: []interface{}{
			outgoingMsg, // Add the specific signaling message structure here
		},
	}

	// Use the existing safe write method
	return uc.WriteJSONSafely(callTrace, fullOutgoingPayload)
}

// Helper to get or create a PeerConnection for a specific callId
// webrtcConfig is passed from the Service
// offer bool indicates if this UC is creating the offer (true) or receiving it (false)
func (uc *UserCtx) GetOrCreatePeerConnection(callTrace []string, callId string, webrtcConfig webrtc.Configuration, isOfferer bool) (*webrtc.PeerConnection, error) {
	uc.webrtcPCsMtx.Lock()
	defer uc.webrtcPCsMtx.Unlock()

	if pc, ok := uc.webrtcPCs[callId]; ok && pc != nil {
		vbl.Stdout.Debug(vbl.Id("vid/webrtc_pc_exists"), fmt.Sprintf("Reusing existing PeerConnection for call %s (user %s)", callId, uc.UserId))
		return pc, nil
	}

	vbl.Stdout.Info(vbl.Id("vid/webrtc_pc_create"), fmt.Sprintf("Creating new PeerConnection for call %s (user %s)", callId, uc.UserId))

	// Create a new PeerConnection
	pc, err := webrtc.NewPeerConnection(webrtcConfig)
	if err != nil {
		vbl.Stdout.Error(vbl.Id("vid/webrtc_pc_create_err"), fmt.Sprintf("Error creating PeerConnection for call %s (user %s): %v", callId, uc.UserId, err))
		return nil, err
	}

	// --- Set up Event Handlers ---

	// Handle ICE candidates generated locally by Pion
	pc.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			// Candidate gathering finished. Optionally send an "ice-completion" signal.
			vbl.Stdout.Debug(vbl.Id("vid/webrtc_ice_candidate_done"), fmt.Sprintf("ICE candidate gathering finished for call %s (user %s)", callId, uc.UserId))
			// Example: Send a signal to the client indicating ICE gathering is complete
			completionPayload := map[string]interface{}{"callId": callId, "status": "completed"}
			if sendErr := uc.SendWebRTCMessage(callTrace, "ice_gathering_status", completionPayload); sendErr != nil {
				vbl.Stdout.Warn(vbl.Id("vid/webrtc_send_ice_gathering_status_err"), fmt.Sprintf("Error sending ICE gathering status for call %s (user %s): %v", callId, uc.UserId, sendErr))
			}
			return
		}

		vbl.Stdout.Debug(vbl.Id("vid/webrtc_ice_candidate"), fmt.Sprintf("New ICE candidate for call %s (user %s): %v", callId, uc.UserId, candidate.ToJSON()))

		// Send the ICE candidate to the other peer via the signaling server (WebSocket)
		// The peer will receive this via the signaling handler in Service.handleIncomingWSMessages
		// The payload must contain the callId for routing on the server and client.
		icePayload := map[string]interface{}{
			"callId":           callId,
			"candidate":        candidate.ToJSON().Candidate, // Raw candidate string
			"sdpMid":           candidate.ToJSON().SDPMid,
			"sdpMLineIndex":    candidate.ToJSON().SDPMLineIndex,
			"originatorUserId": uc.UserId, // Add sender info
		}
		if sendErr := uc.SendWebRTCMessage(callTrace, "ice", icePayload); sendErr != nil {
			vbl.Stdout.Error(vbl.Id("vid/webrtc_send_ice_err"), fmt.Sprintf("Error sending ICE candidate for call %s (user %s): %v", callId, uc.UserId, sendErr))
		}
	})

	// Handle connection state changes
	pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		vbl.Stdout.Info(vbl.Id("vid/webrtc_conn_state"), fmt.Sprintf("PeerConnection state for call %s (user %s) changed: %s", callId, uc.UserId, s.String()))

		// Notify the client over WS about the state change
		statusPayload := map[string]interface{}{"callId": callId, "state": s.String()}
		if sendErr := uc.SendWebRTCMessage(callTrace, "webrtc_connection_status", statusPayload); sendErr != nil {
			vbl.Stdout.Warn(vbl.Id("vid/webrtc_send_conn_status_err"), fmt.Sprintf("Error sending connection status for call %s (user %s): %v", callId, uc.UserId, sendErr))
		}

		switch s {
		case webrtc.PeerConnectionStateConnected:
			vbl.Stdout.Info(vbl.Id("vid/webrtc_connected"), fmt.Sprintf("PeerConnection for call %s (user %s) connected!", callId, uc.UserId))
		case webrtc.PeerConnectionStateDisconnected:
			vbl.Stdout.Warn(vbl.Id("vid/webrtc_disconnected"), fmt.Sprintf("PeerConnection for call %s (user %s) disconnected!", callId, uc.UserId))
			uc.RemovePeerConnection(callId) // Clean up PC on disconnection
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			vbl.Stdout.Error(vbl.Id("vid/webrtc_failed"), fmt.Sprintf("PeerConnection for call %s (user %s) failed or closed!", callId, uc.UserId))
			uc.RemovePeerConnection(callId) // Clean up PC on failure/closure
		default:
			// Other states like New, Connecting
		}
	})

	// Handle ICE connection state changes
	pc.OnICEConnectionStateChange(func(s webrtc.ICEConnectionState) {
		vbl.Stdout.Debug(vbl.Id("vid/webrtc_ice_state"), fmt.Sprintf("ICE connection state for call %s (user %s) changed: %s", callId, uc.UserId, s.String()))
		// Optionally notify the client
		iceStatePayload := map[string]interface{}{"callId": callId, "iceState": s.String()}
		if sendErr := uc.SendWebRTCMessage(callTrace, "ice_status", iceStatePayload); sendErr != nil {
			vbl.Stdout.Warn(vbl.Id("vid/webrtc_send_ice_status_err"), fmt.Sprintf("Error sending ICE status for call %s (user %s): %v", callId, uc.UserId, sendErr))
		}
	})

	// Handle Signaling state changes
	pc.OnSignalingStateChange(func(s webrtc.SignalingState) {
		vbl.Stdout.Debug(vbl.Id("vid/webrtc_signaling_state"), fmt.Sprintf("Signaling state for call %s (user %s) changed: %s", callId, uc.UserId, s.String()))
		// Optionally notify the client
		signalingStatePayload := map[string]interface{}{"callId": callId, "signalingState": s.String()}
		if sendErr := uc.SendWebRTCMessage(callTrace, "signaling_status", signalingStatePayload); sendErr != nil {
			vbl.Stdout.Warn(vbl.Id("vid/webrtc_send_signaling_status_err"), fmt.Sprintf("Error sending signaling status for call %s (user %s): %v", callId, uc.UserId, sendErr))
		}
	})

	// Handle DataChannel creation initiated by the remote peer
	pc.OnDataChannel(func(d *webrtc.DataChannel) {
		vbl.Stdout.Info(vbl.Id("vid/webrtc_data_channel"), fmt.Sprintf("New DataChannel '%s' (ID: %d) for call %s (user %s)", d.Label(), *d.ID(), callId, uc.UserId))

		// Set up DataChannel event handlers
		setupDataChannelHandlers(d, uc, callId, callTrace) // Use a helper function

	})

	// If this peer is the offerer, create a DataChannel
	if isOfferer {
		// Options: &webrtc.DataChannelInit{ Ordered: webrtc.Bool(false), MaxRetransmits: webrtc.Uint16(0) } for unreliable
		// Let's create a reliable, ordered data channel for chat messages by default.
		options := &webrtc.DataChannelInit{
			Ordered:        webrtc.Bool(true),
			MaxRetransmits: webrtc.Uint16(65535), // Default reliable, ordered
		}
		dataChannel, err := pc.CreateDataChannel("chat", options) // Data channel label, options
		if err != nil {
			vbl.Stdout.Error(vbl.Id("vid/webrtc_dc_create_err"), fmt.Sprintf("Error creating DataChannel for call %s (user %s): %v", callId, uc.UserId, err))
			// Return the PC anyway, but log the error. The client might still want audio/video.
		} else {
			vbl.Stdout.Info(vbl.Id("vid/webrtc_dc_created"), fmt.Sprintf("Created DataChannel '%s' (ID: %d) for call %s (user %s)", dataChannel.Label(), *dataChannel.ID(), callId, uc.UserId))
			setupDataChannelHandlers(dataChannel, uc, callId, callTrace) // Setup handlers for the channel we created
		}
	}

	// Store the new PeerConnection in the map
	uc.webrtcPCsMtx.Lock()
	uc.webrtcPCs[callId] = pc
	uc.webrtcPCsMtx.Unlock()

	return pc, nil
}

// setupDataChannelHandlers sets up event handlers for a given DataChannel.
// This is called for channels created locally (offerer) and remotely (answerer).
func setupDataChannelHandlers(d *webrtc.DataChannel, uc *UserCtx, callId string, callTrace []string) {
	d.OnOpen(func() {
		vbl.Stdout.Info(vbl.Id("vid/webrtc_data_channel_open"), fmt.Sprintf("DataChannel '%s' (ID: %d) for call %s (user %s) is now open.", d.Label(), *d.ID(), callId, uc.UserId))
		// Optionally send a message over this DataChannel to signal readiness or send initial data
		// Note: Sending binary []byte("...") or text string("...")
		if sendErr := d.SendText(fmt.Sprintf("Hello from server DataChannel! DC '%s' (ID: %d) for call %s", d.Label(), *d.ID(), callId)); sendErr != nil {
			vbl.Stdout.Warn(vbl.Id("vid/webrtc_dc_send_err"), fmt.Sprintf("Error sending initial DataChannel message for call %s (user %s): %v", callId, uc.UserId, sendErr))
		}
		// Optionally notify the client over WS that the DC is open
		dcStatusPayload := map[string]interface{}{"callId": callId, "dcLabel": d.Label(), "dcID": *d.ID(), "state": "open"}
		if sendErr := uc.SendWebRTCMessage(callTrace, "datachannel_status", dcStatusPayload); sendErr != nil {
			vbl.Stdout.Warn(vbl.Id("vid/webrtc_send_dc_status_err"), fmt.Sprintf("Error sending DataChannel status for call %s (user %s): %v", callId, uc.UserId, sendErr))
		}
	})

	d.OnMessage(func(msg webrtc.DataChannelMessage) {
		// msg.Data is []byte
		if msg.IsString {
			vbl.Stdout.Info(vbl.Id("vid/webrtc_data_channel_msg_text"), fmt.Sprintf("DataChannel '%s' (ID: %d) received text message for call %s (user %s): %s", d.Label(), *d.ID(), callId, uc.UserId, string(msg.Data)))
			// Echo the message back over the DataChannel (example)
			if echoErr := d.SendText("Server received your text: " + string(msg.Data)); echoErr != nil {
				vbl.Stdout.Warn(vbl.Id("vid/webrtc_dc_echo_err"), fmt.Sprintf("Error echoing text DataChannel message for call %s (user %s): %v", callId, uc.UserId, echoErr))
			}
			// TODO: Implement logic to relay this message to other participants in the call
			// This would likely involve finding other UserCtx objects associated with this callId
			// and sending the message to them, either over WS or potentially over their own
			// connected DataChannel if applicable (SFU/MCU).
			// For a simple P2P chat, this message is just between these two clients via their PCs.
			// If the server is signaling only, it doesn't process the *data* here, only the *signaling* messages over WS.
			// If the server is an SFU/MCU, it *does* process the data here. This code assumes it *might* process the data.

			// Example: Relay the message back to the sender over WS as confirmation
			// Or relay to other users in the conversation via WS
			// We need a way to map callId back to the conversation ID and find other users.
			// This requires looking up callId in a service-level map or having the callId associated
			// with a conversation/room.

			// Let's simulate relaying to *other* users in the same call (which might be on different PCs/UCs)
			// This requires access to the Service's connection maps and call management logic.
			// For a true P2P setup where the server only signals, this relaying logic might live
			// entirely on the client-side peers. If the server is involved in relaying data,
			// it's moving towards SFU/MCU architecture.

			// Simple approach for now: just log and perhaps send a confirmation back over WS
			dataRelayPayload := map[string]interface{}{
				"callId":     callId,
				"dcLabel":    d.Label(),
				"fromUserId": uc.UserId,        // Include sender's info
				"message":    string(msg.Data), // The actual message content
				"timestamp":  time.Now().UTC().String(),
			}
			// Send this data message over WS (using the "data" type in signaling)
			if relayErr := uc.SendWebRTCMessage(callTrace, "data", dataRelayPayload); relayErr != nil {
				vbl.Stdout.Warn(vbl.Id("vid/webrtc_relay_ws_err"), fmt.Sprintf("Error relaying data message over WS for call %s (user %s): %v", callId, uc.UserId, relayErr))
			}

		} else {
			vbl.Stdout.Info(vbl.Id("vid/webrtc_data_channel_msg_bin"), fmt.Sprintf("DataChannel '%s' (ID: %d) received binary message (%d bytes) for call %s (user %s)", d.Label(), *d.ID(), len(msg.Data), callId, uc.UserId))
			// Echo the message back over the DataChannel (example)
			if echoErr := d.Send(msg.Data); echoErr != nil {
				vbl.Stdout.Warn(vbl.Id("vid/webrtc_dc_echo_err_bin"), fmt.Sprintf("Error echoing binary DataChannel message for call %s (user %s): %v", callId, uc.UserId, echoErr))
			}
			// TODO: Process or relay binary data
		}
	})

	d.OnClose(func() {
		vbl.Stdout.Info(vbl.Id("vid/webrtc_data_channel_close"), fmt.Sprintf("DataChannel '%s' (ID: %d) for call %s (user %s) is now closed.", d.Label(), *d.ID(), callId, uc.UserId))
		// TODO: Clean up DataChannel reference if stored in UserCtx
		dcStatusPayload := map[string]interface{}{"callId": callId, "dcLabel": d.Label(), "dcID": *d.ID(), "state": "closed"}
		if sendErr := uc.SendWebRTCMessage(callTrace, "datachannel_status", dcStatusPayload); sendErr != nil {
			vbl.Stdout.Warn(vbl.Id("vid/webrtc_send_dc_status_err_close"), fmt.Sprintf("Error sending DataChannel status for call %s (user %s): %v", callId, uc.UserId, sendErr))
		}
	})

	d.OnError(func(err error) {
		vbl.Stdout.Error(vbl.Id("vid/webrtc_data_channel_error"), fmt.Sprintf("DataChannel '%s' (ID: %d) for call %s (user %s) encountered an error: %v", d.Label(), *d.ID(), callId, uc.UserId, err))
		dcStatusPayload := map[string]interface{}{"callId": callId, "dcLabel": d.Label(), "dcID": *d.ID(), "state": "error", "error": err.Error()}
		if sendErr := uc.SendWebRTCMessage(callTrace, "datachannel_status", dcStatusPayload); sendErr != nil {
			vbl.Stdout.Warn(vbl.Id("vid/webrtc_send_dc_status_err_error"), fmt.Sprintf("Error sending DataChannel status for call %s (user %s): %v", callId, uc.UserId, sendErr))
		}
	})
}

// RemovePeerConnection closes and removes a PeerConnection from the UserCtx.
func (uc *UserCtx) RemovePeerConnection(callId string) {
	uc.webrtcPCsMtx.Lock()
	defer uc.webrtcPCsMtx.Unlock()

	if pc, ok := uc.webrtcPCs[callId]; ok && pc != nil {
		vbl.Stdout.Info(vbl.Id("vid/webrtc_pc_remove"), fmt.Sprintf("Removing PeerConnection for call %s (user %s)", callId, uc.UserId))
		// Attempt to close the PC if it's not already closed/failed
		if pc.ConnectionState() != webrtc.PeerConnectionStateClosed && pc.ConnectionState() != webrtc.PeerConnectionStateFailed {
			if closeErr := pc.Close(); closeErr != nil && !errors.Is(closeErr, webrtc.ErrConnectionClosed) {
				vbl.Stdout.Warn(vbl.Id("vid/webrtc_pc_close_on_remove_err"), fmt.Sprintf("Error closing WebRTC PC on remove for call %s (user %s): %v", callId, uc.UserId, closeErr))
			}
		}
		delete(uc.webrtcPCs, callId) // Remove from map
	} else {
		vbl.Stdout.Debug(vbl.Id("vid/webrtc_pc_remove_not_found"), fmt.Sprintf("PeerConnection for call %s not found for removal (user %s)", callId, uc.UserId))
	}
}

// WebsocketOutgoingPayload structure definition (copied from previous diff)
// easyjson:json
type WSOutgoingMeta struct {
	FromSource  string `json:"fromSource"`
	TimeCreated string `json:"timeCreated"`
}

//easyjson:json
type WebsocketOutgoingPayload struct {
	Meta     WSOutgoingMeta `json:"meta"`
	Messages []interface{}  `json:"messages"` // Contains the list of messages/events to the client
}
