package events

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

const (
	TypeRepairOrderCreated  = "repair_order:created"
	TypeEstimateSent        = "estimate:sent"
	TypeEstimateApproved    = "estimate:approved"
	TypeMechanicClockedIn   = "mechanic:clocked_in"
	TypeInventoryLowStock   = "inventory:low_stock"
)

type RepairOrderCreatedPayload struct {
	RepairOrderID string `json:"repair_order_id"`
	ShopID        string `json:"shop_id"`
	CustomerID    string `json:"customer_id"`
}

type EstimateSentPayload struct {
	EstimateID    string `json:"estimate_id"`
	RepairOrderID string `json:"repair_order_id"`
	ShopID        string `json:"shop_id"`
}

type EstimateApprovedPayload struct {
	EstimateID    string `json:"estimate_id"`
	RepairOrderID string `json:"repair_order_id"`
	ShopID        string `json:"shop_id"`
}

type MechanicClockedInPayload struct {
	MechanicID    string `json:"mechanic_id"`
	RepairOrderID string `json:"repair_order_id"`
	ShopID        string `json:"shop_id"`
}

type InventoryLowStockPayload struct {
	PartID string `json:"part_id"`
	ShopID string `json:"shop_id"`
	Name   string `json:"name"`
	Stock  int    `json:"stock"`
}

// realtimeChannelPrefix namespaces the per-shop Redis pub/sub channels that
// carry events to connected SSE clients.
const realtimeChannelPrefix = "realtime:shop:"

// RealtimeChannel is the channel a shop's events are published on.
func RealtimeChannel(shopID string) string { return realtimeChannelPrefix + shopID }

// RealtimeChannelPattern matches every shop's channel, for a single PSubscribe.
func RealtimeChannelPattern() string { return realtimeChannelPrefix + "*" }

// ShopFromRealtimeChannel recovers the shop id from a channel name, or "" if
// the name is not one of ours.
func ShopFromRealtimeChannel(channel string) string {
	if !strings.HasPrefix(channel, realtimeChannelPrefix) {
		return ""
	}
	return strings.TrimPrefix(channel, realtimeChannelPrefix)
}

// Envelope is the wire format delivered to SSE clients: the event type plus the
// original payload, so a client can dispatch without guessing from shape.
type Envelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type Bus struct {
	client *asynq.Client
	rdb    *redis.Client
}

func NewBus(rdb *redis.Client) *Bus {
	return &Bus{
		client: asynq.NewClient(asynq.RedisClientOpt{Addr: rdb.Options().Addr}),
		rdb:    rdb,
	}
}

func (b *Bus) Publish(ctx context.Context, eventType string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Mirror onto pub/sub for live clients. Kept separate from the asynq
	// enqueue on purpose: asynq hands a task to exactly one worker, which is
	// right for background jobs and wrong for fanout — with more than one API
	// instance, only one would ever see it. Best effort; a realtime update that
	// does not arrive must never fail the request that caused it.
	if channel, msg, ok := realtimeMessage(eventType, data); ok {
		b.rdb.Publish(ctx, channel, msg)
	}

	task := asynq.NewTask(eventType, data)
	_, err = b.client.Enqueue(task)
	return err
}

// realtimeMessage builds the channel and wire payload for an event, reporting
// false when the payload carries no shop and so cannot be routed to anyone.
// Split out from Publish so it is testable without a Redis server.
func realtimeMessage(eventType string, data []byte) (string, []byte, bool) {
	var envelope struct {
		ShopID string `json:"shop_id"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil || envelope.ShopID == "" {
		return "", nil, false
	}
	msg, err := json.Marshal(Envelope{Type: eventType, Data: data})
	if err != nil {
		return "", nil, false
	}
	return RealtimeChannel(envelope.ShopID), msg, true
}
