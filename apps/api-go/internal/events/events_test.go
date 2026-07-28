package events

import (
	"encoding/json"
	"testing"
)

func TestRealtimeChannelRoundTrip(t *testing.T) {
	channel := RealtimeChannel("shop-1")

	if got := ShopFromRealtimeChannel(channel); got != "shop-1" {
		t.Errorf("shop = %q, want shop-1", got)
	}
	// A stray channel from something else sharing the Redis instance must not
	// be mistaken for a shop, or it would be fanned out to clients.
	if got := ShopFromRealtimeChannel("asynq:queues"); got != "" {
		t.Errorf("shop = %q, want empty for a foreign channel", got)
	}
}

func TestRealtimeMessageWrapsPayload(t *testing.T) {
	data, err := json.Marshal(RepairOrderCreatedPayload{
		RepairOrderID: "ro-1",
		ShopID:        "shop-1",
		CustomerID:    "cust-1",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	channel, msg, ok := realtimeMessage(TypeRepairOrderCreated, data)
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if want := RealtimeChannel("shop-1"); channel != want {
		t.Errorf("channel = %q, want %q", channel, want)
	}

	var envelope Envelope
	if err := json.Unmarshal(msg, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if envelope.Type != TypeRepairOrderCreated {
		t.Errorf("type = %q, want %q", envelope.Type, TypeRepairOrderCreated)
	}

	var payload RepairOrderCreatedPayload
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if payload.RepairOrderID != "ro-1" {
		t.Errorf("repair_order_id = %q, want ro-1", payload.RepairOrderID)
	}
}

// Without a shop there is no room to deliver to. Publishing anyway would either
// panic or leak one shop's event onto a channel nobody owns.
func TestRealtimeMessageSkipsPayloadWithoutShop(t *testing.T) {
	for _, tc := range []struct {
		name string
		data string
	}{
		{"no shop_id", `{"repair_order_id":"ro-1"}`},
		{"empty shop_id", `{"shop_id":""}`},
		{"not an object", `"nope"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, ok := realtimeMessage(TypeRepairOrderCreated, []byte(tc.data)); ok {
				t.Error("ok = true, want false")
			}
		})
	}
}
