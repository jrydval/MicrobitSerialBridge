package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
)

// Message represents a message from a micro:bit
type Message struct {
	Type         string    `json:"type"`          // V (variable), S (string), N (number)
	SerialNumber string    `json:"serial_number"` // Serial number of the sending micro:bit (FC641EBE)
	RSSI         string    `json:"rssi"`          // Signal strength at the receiving bridge (-59 dBm)
	SenderUptime string    `json:"sender_uptime"` // Uptime of the sending micro:bit in ms (538953)
	Payload      string    `json:"payload"`       // Message content (Running, var=1000, 2025)
	ReceivedAt   time.Time `json:"received_at"`   // Time the message was received by the bridge
}

// ParseMessage parses a line in the format |type|serial_number|RSSI|uptime|message|
func ParseMessage(line string) (*Message, error) {
	line = strings.TrimSpace(line)

	// Remove leading and trailing |
	line = strings.Trim(line, "|")

	// Split by |
	parts := strings.Split(line, "|")

	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid message format: expected 5 parts, got %d", len(parts))
	}

	return &Message{
		Type:         parts[0],
		SerialNumber: parts[1],
		RSSI:         parts[2],
		SenderUptime: parts[3],
		Payload:      parts[4],
		ReceivedAt:   time.Now().UTC(),
	}, nil
}

// WebhookSender sends messages as JSON to the configured webhook URL
type WebhookSender struct {
	URL    string
	Client *http.Client
}

// NewWebhookSender creates a new sender with a default HTTP timeout
func NewWebhookSender(url string) *WebhookSender {
	return &WebhookSender{
		URL:    url,
		Client: &http.Client{Timeout: 5 * time.Second},
	}
}

// Send serializes the message to JSON and sends it as a POST to the webhook URL
func (w *WebhookSender) Send(msg *Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("error serializing message to JSON: %w", err)
	}

	resp, err := w.Client.Post(w.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("error sending to webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned an error status: %s", resp.Status)
	}

	return nil
}

// FlexibleValue represents a payload value that in JSON can arrive as
// either a string or a number (typically for "N" type messages) and is always stored as text.
type FlexibleValue string

// UnmarshalJSON accepts either a JSON string or a JSON number and stores it as text.
func (f *FlexibleValue) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = FlexibleValue(s)
		return nil
	}

	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*f = FlexibleValue(n.String())
		return nil
	}

	return fmt.Errorf("field \"payload\" must be a string or a number")
}

// OutgoingMessage represents a message received via HTTP that should be sent to the serial port
type OutgoingMessage struct {
	Type    string        `json:"type"`
	Payload FlexibleValue `json:"payload"`
}

// SerialSender sends messages to the serial port in the format |type|payload|
type SerialSender struct {
	port serial.Port
	mu   sync.Mutex
}

// NewSerialSender creates a new sender over the given serial port
func NewSerialSender(port serial.Port) *SerialSender {
	return &SerialSender{port: port}
}

// Send serializes the message into the format |type|payload| and writes it to the serial port
func (s *SerialSender) Send(msg OutgoingMessage) error {
	if msg.Type == "" {
		return fmt.Errorf("field \"type\" is required")
	}
	if strings.Contains(msg.Type, "|") || strings.Contains(string(msg.Payload), "|") {
		return fmt.Errorf("fields \"type\" and \"payload\" must not contain the '|' character")
	}

	line := fmt.Sprintf("|%s|%s|\n", msg.Type, msg.Payload)

	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.port.Write([]byte(line))
	return err
}

// handleSend processes an HTTP POST request with a JSON body {"type": "...", "payload": "..."}
// and sends the resulting message to the serial port
func handleSend(sender *SerialSender) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "only the POST method is allowed", http.StatusMethodNotAllowed)
			return
		}

		var msg OutgoingMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
			return
		}

		if err := sender.Send(msg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		fmt.Printf("→ Sent to serial port: |%s|%s|\n", msg.Type, msg.Payload)
		w.WriteHeader(http.StatusAccepted)
	}
}

// ProcessMessage processes a message (add your own logic here)
func ProcessMessage(msg *Message) {
	uptimeSeconds := parseUptime(msg.SenderUptime)

	fmt.Printf("[%s] Micro:bit S/N: %s | RSSI: %s dBm | Uptime: %s (%s)\n",
		msg.Type, msg.SerialNumber, msg.RSSI, msg.SenderUptime, uptimeSeconds)

	// Process according to message type
	switch msg.Type {
	case "S":
		// String message
		fmt.Printf("  📝 String: %s\n", msg.Payload)

	case "V":
		// Variable - format name=value
		if strings.Contains(msg.Payload, "=") {
			parts := strings.SplitN(msg.Payload, "=", 2)
			fmt.Printf("  📊 Variable: %s = %s\n", parts[0], parts[1])
		} else {
			fmt.Printf("  📊 Value: %s\n", msg.Payload)
		}

	case "N":
		// Numeric value
		fmt.Printf("  🔢 Number: %s\n", msg.Payload)

	default:
		fmt.Printf("  ❓ Unknown message type: %s\n", msg.Type)
		fmt.Printf("  📦 Data: %s\n", msg.Payload)
	}

	fmt.Println()
}

// parseUptime converts uptime in milliseconds to a readable format
func parseUptime(uptimeMs string) string {
	// Attempt to parse as a number
	var ms int64
	_, err := fmt.Sscanf(uptimeMs, "%d", &ms)
	if err != nil {
		return "?"
	}

	seconds := ms / 1000
	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours%24, minutes%60, seconds%60)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes%60, seconds%60)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds%60)
	} else {
		return fmt.Sprintf("%ds", seconds)
	}
}

func main() {
	// Command-line parameters
	portName := flag.String("port", "/dev/tty.usbmodem3102", "Serial port name")
	baudRate := flag.Int("baud", 115200, "Baud rate")
	webhookURL := flag.String("webhook-url", os.Getenv("MICROBIT_WEBHOOK_URL"), "URL of the webhook to which messages are sent as JSON (can also be set via MICROBIT_WEBHOOK_URL)")
	listenAddr := flag.String("listen-addr", ":8081", "Address on which the bridge listens for HTTP requests to send a message to the serial port (empty = disabled)")
	flag.Parse()

	var webhook *WebhookSender
	if *webhookURL != "" {
		webhook = NewWebhookSender(*webhookURL)
		fmt.Printf("Messages will be forwarded to webhook: %s\n", *webhookURL)
	} else {
		fmt.Println("Webhook is not configured, messages are only printed to the console.")
	}

	fmt.Printf("Opening serial port: %s (baud rate: %d)\n", *portName, *baudRate)

	// Serial port configuration
	mode := &serial.Mode{
		BaudRate: *baudRate,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	// Open the port
	port, err := serial.Open(*portName, mode)
	if err != nil {
		log.Fatalf("Error opening port: %v", err)
	}
	defer port.Close()

	// Set the read timeout
	err = port.SetReadTimeout(time.Second * 1)
	if err != nil {
		log.Fatalf("Error setting timeout: %v", err)
	}

	// Start the HTTP server for receiving messages to be sent to the serial port
	if *listenAddr != "" {
		sender := NewSerialSender(port)
		mux := http.NewServeMux()
		mux.HandleFunc("/", handleSend(sender))

		go func() {
			fmt.Printf("HTTP server for sending messages is listening on %s\n", *listenAddr)
			if err := http.ListenAndServe(*listenAddr, mux); err != nil {
				log.Fatalf("HTTP server error: %v", err)
			}
		}()
	}

	fmt.Println("Waiting for messages from micro:bit...")
	fmt.Println("Press Ctrl+C to quit")
	fmt.Println(strings.Repeat("-", 80))

	// Create a scanner for reading line by line
	scanner := bufio.NewScanner(port)

	// Read and process messages
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Parse the message
		msg, err := ParseMessage(line)
		if err != nil {
			fmt.Printf("Parse error: %v (line: %s)\n", err, line)
			continue
		}

		// Process the message
		ProcessMessage(msg)

		// Forward to the webhook, if configured
		if webhook != nil {
			if err := webhook.Send(msg); err != nil {
				fmt.Printf("Error sending to webhook: %v\n", err)
			}
		}
	}

	// Check for a scanning error
	if err := scanner.Err(); err != nil {
		log.Fatalf("Error reading from serial port: %v", err)
	}
}
