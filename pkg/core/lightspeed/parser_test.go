package lightspeed

import (
	"encoding/json"
	"testing"
)

func TestParseLightSpeedDataFromHTML(t *testing.T) {
	sampleHTML := `
<!DOCTYPE html>
<html>
<head>
<script>
{"require":[["ScheduledServerJS","handle",null,[{"mailbox":{"threads":[]}}]]]}
</script>
</head>
<body>
<div>Messenger Content</div>
</body>
</html>
`
	results := ParseLightSpeedDataFromHTML(sampleHTML)
	if len(results) != 1 {
		t.Fatalf("expected 1 lightspeed script block, got %d", len(results))
	}

	reqField, ok := results[0]["require"]
	if !ok || reqField == nil {
		t.Errorf("expected 'require' field in extracted payload")
	}
}

func TestParseLightSpeedDataFromHTMLEmpty(t *testing.T) {
	sampleHTML := "<html><body>No lightspeed scripts here</body></html>"
	results := ParseLightSpeedDataFromHTML(sampleHTML)
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestExtractLightSpeedThreadsAndContacts(t *testing.T) {
	innerPayload, _ := json.Marshal(map[string]any{
		"step": []any{
			[]any{5, "verifyContactRowExists", "1001", "", "https://avatar.test", "Test User"},
			[]any{5, "updateOrInsertThread", "1700000000", nil, nil, nil, nil, nil, nil, "1001", nil, "1", nil, "inbox", nil, nil, nil, nil, nil, nil, nil, nil, nil, "1001"},
		},
	})
	outerPayload, _ := json.Marshal(map[string]any{
		"lightspeed_web_request": map[string]any{
			"payload": string(innerPayload),
		},
	})
	sampleHTML := "<script type=\"application/json\">" + string(outerPayload) + "</script>"

	threads, contacts := ExtractLightSpeedThreadsAndContacts(sampleHTML)
	if len(contacts) != 1 {
		t.Fatalf("expected 1 contact, got %d", len(contacts))
	}
	c, ok := contacts["1001"]
	if !ok || c.Name != "Test User" {
		t.Errorf("unexpected contact: %+v", c)
	}

	if len(threads) != 1 {
		t.Fatalf("expected 1 thread, got %d", len(threads))
	}
	if threads[0].ThreadID != "1001" || threads[0].Name != "Test User" {
		t.Errorf("unexpected thread: %+v", threads[0])
	}
}
