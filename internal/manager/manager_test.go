package manager

import (
	"net/url"
	"testing"
)

func TestInjectUTMParams(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		campUUID string
		metadata map[string]string
		wantUTM  map[string]string
	}{
		{
			name:     "basic URL gets all UTM params",
			rawURL:   "https://example.com/product/123",
			campUUID: "abc-def-123",
			metadata: nil,
			wantUTM: map[string]string{
				"utm_source":   "listmonk",
				"utm_medium":   "email",
				"utm_campaign": "abc-def-123",
			},
		},
		{
			name:     "metadata generates utm_content",
			rawURL:   "https://example.com/shop",
			campUUID: "camp-uuid-1",
			metadata: map[string]string{"section": "hero", "cta": "shop"},
			wantUTM: map[string]string{
				"utm_source":   "listmonk",
				"utm_medium":   "email",
				"utm_campaign": "camp-uuid-1",
				"utm_content":  "cta_shop_section_hero",
			},
		},
		{
			name:     "preserves existing UTM params",
			rawURL:   "https://example.com/?utm_source=google&utm_medium=cpc",
			campUUID: "camp-uuid-2",
			metadata: nil,
			wantUTM: map[string]string{
				"utm_source":   "google",
				"utm_medium":   "cpc",
				"utm_campaign": "camp-uuid-2",
			},
		},
		{
			name:     "preserves existing query params",
			rawURL:   "https://example.com/page?ref=email&id=42",
			campUUID: "camp-uuid-3",
			metadata: nil,
			wantUTM: map[string]string{
				"utm_source":   "listmonk",
				"utm_medium":   "email",
				"utm_campaign": "camp-uuid-3",
			},
		},
		{
			name:     "handles URL with fragment",
			rawURL:   "https://example.com/page#section",
			campUUID: "camp-uuid-4",
			metadata: nil,
			wantUTM: map[string]string{
				"utm_source":   "listmonk",
				"utm_medium":   "email",
				"utm_campaign": "camp-uuid-4",
			},
		},
		{
			name:     "empty metadata does not set utm_content",
			rawURL:   "https://example.com/",
			campUUID: "camp-uuid-5",
			metadata: map[string]string{},
			wantUTM: map[string]string{
				"utm_source":   "listmonk",
				"utm_medium":   "email",
				"utm_campaign": "camp-uuid-5",
			},
		},
		{
			name:     "invalid URL returns original",
			rawURL:   "://invalid",
			campUUID: "camp-uuid-6",
			metadata: nil,
			wantUTM:  nil, // returns original
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := injectUTMParams(tt.rawURL, tt.campUUID, tt.metadata)

			if tt.wantUTM == nil {
				if result != tt.rawURL {
					t.Errorf("expected original URL %q, got %q", tt.rawURL, result)
				}
				return
			}

			u, err := url.Parse(result)
			if err != nil {
				t.Fatalf("failed to parse result URL: %v", err)
			}

			q := u.Query()
			for key, want := range tt.wantUTM {
				got := q.Get(key)
				if got != want {
					t.Errorf("UTM param %q: want %q, got %q", key, want, got)
				}
			}
		})
	}
}

func TestInjectUTMParams_PreservesExistingQueryParams(t *testing.T) {
	result := injectUTMParams("https://example.com/page?ref=email&id=42", "camp-1", nil)
	u, err := url.Parse(result)
	if err != nil {
		t.Fatalf("failed to parse result URL: %v", err)
	}
	q := u.Query()
	if q.Get("ref") != "email" {
		t.Errorf("existing query param 'ref' was lost")
	}
	if q.Get("id") != "42" {
		t.Errorf("existing query param 'id' was lost")
	}
}

func TestParseLinkTags(t *testing.T) {
	tests := []struct {
		input string
		want  map[string]string
	}{
		{"section:hero,cta:shop", map[string]string{"section": "hero", "cta": "shop"}},
		{"section:hero", map[string]string{"section": "hero"}},
		{"", map[string]string{}},
		{" section : hero , cta : shop ", map[string]string{"section": "hero", "cta": "shop"}},
	}

	for _, tt := range tests {
		result := parseLinkTags(tt.input)
		for k, v := range tt.want {
			if result[k] != v {
				t.Errorf("parseLinkTags(%q): key %q: want %q, got %q", tt.input, k, v, result[k])
			}
		}
	}
}
