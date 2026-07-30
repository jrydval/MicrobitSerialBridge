# MicrobitSerialBridge

Bridge a [BBC micro:bit](https://microbit.org/) radio network into the wider IT world — Home Assistant, webhooks, whatever speaks HTTP and JSON.

One micro:bit acts as a **radio gateway**: it listens on the micro:bit radio, and forwards every packet it hears out over its USB serial line. A small Go service running next to it (typically in Docker) reads that serial stream, turns each message into JSON, and POSTs it to a configurable webhook. The same path works in reverse — POST a JSON message to the bridge's HTTP endpoint and it gets written to the gateway micro:bit's serial port, which then broadcasts it out over radio to the rest of the micro:bit swarm.

```
other micro:bits  --radio-->  gateway micro:bit  --USB/serial-->  Go bridge  --HTTP webhook-->  Home Assistant / anything
other micro:bits  <--radio--  gateway micro:bit  <--USB/serial--  Go bridge  <--HTTP POST-------  Home Assistant / anything
```

## Components

| File | Role |
|---|---|
| [`microbit-serial-bridge.ts`](microbit-serial-bridge.ts) | MakeCode/TypeScript firmware for the **gateway micro:bit**. Relays radio traffic to serial and serial commands to radio. |
| [`microbit_serial.go`](microbit_serial.go) | The **Go bridge**. Reads the gateway's serial output, parses it, forwards it to a webhook, and exposes an HTTP endpoint to send messages back. |
| [`Dockerfile`](Dockerfile) / [`docker-compose.yml`](docker-compose.yml) | Container build/run setup for the Go bridge. |
| [`test_receiver.py`](test_receiver.py) | A tiny local HTTP server that prints whatever the bridge sends it — useful for testing without Home Assistant. |
| `sample_*` | Example wiring for a Home Assistant integration (see below). |

## Message format

The gateway micro:bit and the bridge talk over serial using a simple pipe-delimited line protocol.

**Radio → serial** (gateway forwards a received radio packet):

```
|type|serial_number|rssi|sender_uptime|payload|
```

- `type` — `N` (number), `S` (string), or `V` (named value)
- `serial_number` — hex serial number of the sending micro:bit
- `rssi` — signal strength in dBm
- `sender_uptime` — sender's uptime in ms
- `payload` — the message content (e.g. `Running`, `2025`, `SOCKET1=1`)

The Go bridge turns this into JSON and POSTs it to the configured webhook:

```json
{
  "type": "V",
  "serial_number": "FC641EBE",
  "rssi": "-59",
  "sender_uptime": "538953",
  "payload": "SOCKET1=1",
  "received_at": "2026-07-30T20:12:00Z"
}
```

**Serial → radio** (send a message out over radio):

```
|type|payload|
```

The bridge's HTTP endpoint accepts the same shape as JSON:

```bash
curl -X POST http://localhost:8081/ \
  -H 'Content-Type: application/json' \
  -d '{"type": "S", "payload": "ON"}'
```

which the gateway micro:bit then broadcasts via `radio.sendString` (or `sendNumber`/`sendValue`, depending on `type`).

## Getting started

### 1. Flash the gateway micro:bit

Open [`microbit-serial-bridge.ts`](microbit-serial-bridge.ts) in the [MakeCode editor](https://makecode.microbit.org/) and flash it onto the micro:bit that will stay plugged in via USB. See [LED display](#led-display) below for what the lights mean.

### 2. Run the bridge

With Go installed:

```bash
go run . -port=/dev/ttyACM0 -baud=115200 -webhook-url=http://localhost:8080 -listen-addr=:8081
```

Or with Docker Compose:

```bash
cp .env.example .env   # edit SERIAL_PORT / MICROBIT_WEBHOOK_URL as needed
docker compose up --build
```

### 3. (Optional) Try it locally with the test receiver

```bash
python3 test_receiver.py 8080
```

Point `-webhook-url` (or `MICROBIT_WEBHOOK_URL`) at `http://localhost:8080` and every radio message the gateway hears will be printed as formatted JSON.

## LED display

The gateway micro:bit uses its 5×5 LED display as a simple activity monitor, split into three zones:

```
row 0  ●  ·  ·  ·  ·     ┐
row 1  ·  ·  ·  ·  ·     ┘  receive  (radio → serial)
row 2  ·  ·  ·  ·  ·     ┐
row 3  ·  ·  ·  ·  ·     ┘  send     (serial → radio)
row 4  ·  ·  ·  ·  ◆        heartbeat (blinks every second, proves the firmware is alive)
```

- **Rows 0–1 (receive):** each radio packet the gateway hears (and forwards to serial) toggles the next LED in a sweep across these two rows, so a busy radio network shows up as flickering across the top of the display.
- **Rows 2–3 (send):** each line the gateway reads from serial (and broadcasts out over radio) toggles the next LED across these two rows in the same way — useful for confirming that commands sent from the Go bridge are actually reaching the micro:bit.
- **Row 4, bottom-right pixel:** blinks once a second regardless of traffic, as a heartbeat to confirm the gateway is powered and the firmware hasn't hung.

## Configuration

| Flag | Env var | Default | Description |
|---|---|---|---|
| `-port` | — | `/dev/tty.usbmodem3102` | Serial port the gateway micro:bit is connected to |
| `-baud` | — | `115200` | Serial baud rate |
| `-webhook-url` | `MICROBIT_WEBHOOK_URL` | — | Webhook URL that receives every message as JSON |
| `-listen-addr` | — | `:8081` | Address the bridge listens on for outgoing messages (empty disables it) |

## Home Assistant example

The `sample_*` files show a full round trip: a second micro:bit sends a radio message to toggle a light, Home Assistant flips the switch, and sends back `"ON"`/`"OFF"` which is shown on that micro:bit's display.

- [`sample_radio_sender.ts`](sample_radio_sender.ts) — firmware for the second micro:bit. Button A sends `SOCKET1=0`, button B sends `SOCKET1=1`, and any string/number/value received back is shown on the display.
- [`sample_docker-compose.yaml`](sample_docker-compose.yaml) — bridge configured to forward messages straight to a Home Assistant webhook.
- [`sample_has_configuration_fragment.yaml`](sample_has_configuration_fragment.yaml) — a `rest_command` in Home Assistant's `configuration.yaml` for sending messages back to the bridge.
- [`sample_automation.yaml`](sample_automation.yaml) — a Home Assistant automation triggered by the bridge's webhook: it turns `switch.socket_1` on or off depending on the payload, then calls `rest_command.microbit_send` to report `"ON"`/`"OFF"` back over radio.

Wire the pieces together and you can flip a physical switch on one micro:bit and see the actual light state reflected back on it a moment later — no Wi-Fi chip required on either device.
