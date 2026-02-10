package posthog

import (
	"encoding/json"
	"testing"

	"github.com/knadh/listmonk/models"
)

func TestParsePayload_SingleEvent(t *testing.T) {
	raw := `{"event": "product_viewed", "distinct_id": "user@example.com", "properties": {"product_name": "Widget"}, "timestamp": "2026-02-10T12:00:00Z"}`

	events, err := ParsePayload([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Event != "product_viewed" {
		t.Errorf("expected event 'product_viewed', got %q", events[0].Event)
	}
	if events[0].DistinctID != "user@example.com" {
		t.Errorf("expected distinct_id 'user@example.com', got %q", events[0].DistinctID)
	}
}

func TestParsePayload_Array(t *testing.T) {
	raw := `[
		{"event": "product_viewed", "distinct_id": "user@example.com", "properties": {}, "timestamp": "2026-02-10T12:00:00Z"},
		{"event": "order_completed", "distinct_id": "user@example.com", "properties": {"revenue": 99.99}, "timestamp": "2026-02-10T12:01:00Z"}
	]`

	events, err := ParsePayload([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestParsePayload_BatchWithData(t *testing.T) {
	raw := `{"data": [
		{"event": "$pageview", "distinct_id": "anon-123", "properties": {}, "timestamp": "2026-02-10T12:00:00Z"}
	]}`

	events, err := ParsePayload([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Event != "$pageview" {
		t.Errorf("expected event '$pageview', got %q", events[0].Event)
	}
}

func TestParsePayload_Invalid(t *testing.T) {
	_, err := ParsePayload([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParsePayload_EmptyArray(t *testing.T) {
	_, err := ParsePayload([]byte(`[]`))
	if err == nil {
		t.Error("expected error for empty array")
	}
}

func TestParsePayload_EmptyDataField(t *testing.T) {
	_, err := ParsePayload([]byte(`{"data": []}`))
	if err == nil {
		t.Error("expected error for empty data array")
	}
}

func TestExtractEmail(t *testing.T) {
	tests := []struct {
		name  string
		event models.PostHogEvent
		want  string
	}{
		{
			name: "from $set.email",
			event: models.PostHogEvent{
				DistinctID: "anon-123",
				Properties: map[string]any{
					"$set": map[string]any{"email": "User@Example.com"},
				},
			},
			want: "user@example.com",
		},
		{
			name: "from properties.email",
			event: models.PostHogEvent{
				DistinctID: "anon-456",
				Properties: map[string]any{
					"email": "Test@Example.COM",
				},
			},
			want: "test@example.com",
		},
		{
			name: "from distinct_id when email format",
			event: models.PostHogEvent{
				DistinctID: "buyer@kencove.com",
				Properties: map[string]any{},
			},
			want: "buyer@kencove.com",
		},
		{
			name: "no email found",
			event: models.PostHogEvent{
				DistinctID: "anon-789",
				Properties: map[string]any{},
			},
			want: "",
		},
		{
			name: "invalid email in distinct_id",
			event: models.PostHogEvent{
				DistinctID: "not-an-email",
				Properties: map[string]any{},
			},
			want: "",
		},
		{
			name: "$set.email takes priority",
			event: models.PostHogEvent{
				DistinctID: "other@example.com",
				Properties: map[string]any{
					"$set":  map[string]any{"email": "primary@example.com"},
					"email": "fallback@example.com",
				},
			},
			want: "primary@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractEmail(tt.event)
			if got != tt.want {
				t.Errorf("ExtractEmail(): want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestExtractCampaignUUID(t *testing.T) {
	tests := []struct {
		name  string
		event models.PostHogEvent
		want  string
	}{
		{
			name: "from $current_url utm_campaign",
			event: models.PostHogEvent{
				Properties: map[string]any{
					"$current_url": "https://kencove.com/product/123?utm_campaign=550e8400-e29b-41d4-a716-446655440000&utm_source=listmonk",
				},
			},
			want: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name: "from utm_campaign property",
			event: models.PostHogEvent{
				Properties: map[string]any{
					"utm_campaign": "550e8400-e29b-41d4-a716-446655440000",
				},
			},
			want: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name: "from $initial_utm_campaign",
			event: models.PostHogEvent{
				Properties: map[string]any{
					"$initial_utm_campaign": "550e8400-e29b-41d4-a716-446655440000",
				},
			},
			want: "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name: "no campaign UUID",
			event: models.PostHogEvent{
				Properties: map[string]any{
					"$current_url": "https://kencove.com/product/123",
				},
			},
			want: "",
		},
		{
			name: "non-UUID utm_campaign ignored",
			event: models.PostHogEvent{
				Properties: map[string]any{
					"utm_campaign": "spring-sale-2026",
				},
			},
			want: "",
		},
		{
			name: "empty properties",
			event: models.PostHogEvent{
				Properties: map[string]any{},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractCampaignUUID(tt.event)
			if got != tt.want {
				t.Errorf("ExtractCampaignUUID(): want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestExtractRevenue(t *testing.T) {
	tests := []struct {
		name  string
		event models.PostHogEvent
		want  float64
	}{
		{
			name: "order_completed with revenue float",
			event: models.PostHogEvent{
				Event:      models.PostHogEventOrderCompleted,
				Properties: map[string]any{"revenue": 99.99},
			},
			want: 99.99,
		},
		{
			name: "order_completed with total fallback",
			event: models.PostHogEvent{
				Event:      models.PostHogEventOrderCompleted,
				Properties: map[string]any{"total": 150.00},
			},
			want: 150.00,
		},
		{
			name: "non-order event returns 0",
			event: models.PostHogEvent{
				Event:      models.PostHogEventProductViewed,
				Properties: map[string]any{"revenue": 50.00},
			},
			want: 0,
		},
		{
			name: "order_completed with no revenue",
			event: models.PostHogEvent{
				Event:      models.PostHogEventOrderCompleted,
				Properties: map[string]any{},
			},
			want: 0,
		},
		{
			name: "json.Number type not matched",
			event: models.PostHogEvent{
				Event:      models.PostHogEventOrderCompleted,
				Properties: map[string]any{"revenue": json.Number("42")},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractRevenue(tt.event)
			if got != tt.want {
				t.Errorf("ExtractRevenue(): want %f, got %f", tt.want, got)
			}
		})
	}
}
