package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Fyve-Labs/tessa-daemon/internal/pubsub"
)

// webhookRequest is the request payload for /api/v1/webhook
//
//	{
//	  "iot_thing_name": "<string>",
//	  "mqtt_topic_name": "<string>",
//	  "mqtt_event_payload": <any json>
//	}
//
// Kept internal to the server package.
type webhookRequest struct {
	IoTThingName     string          `json:"iot_thing_name"`
	MQTTTopicName    string          `json:"mqtt_topic_name"`
	MQTTEventPayload json.RawMessage `json:"mqtt_event_payload"`
}

// apiServer implements http.Handler and wires the routes.
type apiServer struct {
	ps  pubsub.PubSub
	mux *http.ServeMux
}

// New constructs the API server and returns it as an http.Handler.
func New(ps pubsub.PubSub) http.Handler {
	mux := http.NewServeMux()
	srv := &apiServer{ps: ps, mux: mux}
	mux.HandleFunc("/api/v1/webhook", srv.handleWebhook)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return srv
}

func (s *apiServer) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *apiServer) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "application/json" && ct != "application/json; charset=utf-8" {
		// Be lenient but require application/json prefix
		if ct == "" || len(ct) < len("application/json") || ct[:len("application/json")] != "application/json" {
			http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
			return
		}
	}
	var req webhookRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid json: %v", err), http.StatusBadRequest)
		return
	}
	if req.MQTTTopicName == "" {
		http.Error(w, "missing field: mqtt_topic_name", http.StatusBadRequest)
		return
	}
	if req.IoTThingName == "" {
		http.Error(w, "missing field: iot_thing_name", http.StatusBadRequest)
		return
	}
	// Derive the routed topic by removing the thing name segment, if present.
	topic := normalizeMQTTTopic(req.MQTTTopicName, req.IoTThingName)
	msg := pubsub.Message{Topic: topic, Data: req.MQTTEventPayload, Time: time.Now().UTC(), IoTThingName: req.IoTThingName}
	if err := s.ps.Publish(topic, msg); err != nil {
		http.Error(w, fmt.Sprintf("failed to publish: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "queued",
		"topic":   topic,
		"time":    msg.Time,
		"message": "published",
	})
}

// normalizeMQTTTopic removes the specific iot thing name from the topic path if present.
// Examples:
//   - "$aws/things/device123/tunnels/notify" + thing "device123" => "$aws/things/tunnels/notify"
//   - "tessa/things/device123/commands/notify" + thing "device123" => "tessa/things/commands/notify"
func normalizeMQTTTopic(topic, thing string) string {
	if thing == "" {
		return topic
	}
	needle := "/things/" + thing
	if idx := strings.Index(topic, needle); idx != -1 {
		return strings.Replace(topic, needle, "/things", 1)
	}

	return topic
}

// PrepareUnixSocket ensures the directory exists, removes any stale socket, and listens on the given path.
func PrepareUnixSocket(path string) (net.Listener, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	// Remove stale socket
	if _, err := os.Stat(path); err == nil {
		_ = os.Remove(path)
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen unix %s: %w", path, err)
	}
	// Set permissive permissions; tighten later if needed
	_ = os.Chmod(path, 0o666)
	return ln, nil
}
