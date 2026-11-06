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

func TestExtractHybridDirectThreadUsesRecipientMapping(t *testing.T) {
	threadArgs := make([]any, 90)
	threadArgs[0] = "1793795130303"
	threadArgs[7] = "thread_9001"
	threadArgs[9] = "15"
	threadArgs[10] = "inbox"

	innerPayload, _ := json.Marshal(map[string]any{
		"step": []any{
			[]any{5, "verifyContactRowExists", "recipient_42", "", "https://avatar.test/recipient", "Persona real"},
			[]any{5, "updateThreadAuthorityAndMappingWithOTIDFromJID", "recipient_42", "thread_9001"},
			[]any{5, "verifyHybridThreadExists", "thread_9001", "recipient_42", "15"},
			append([]any{5, "updateOrInsertThread"}, threadArgs...),
		},
	})
	outerPayload, _ := json.Marshal(map[string]any{
		"lightspeed_web_request": map[string]any{"payload": string(innerPayload)},
	})

	threads, _ := ExtractLightSpeedThreadsAndContacts("<script>" + string(outerPayload) + "</script>")
	if len(threads) != 1 {
		t.Fatalf("expected 1 thread, got %d", len(threads))
	}
	got := threads[0]
	if got.IsGroup || got.Type != "user" || !got.IsE2EE {
		t.Fatalf("thread type 15 must be an E2EE direct thread: %+v", got)
	}
	if got.Name != "Persona real" || got.RecipientID != "recipient_42" || got.ChatJID != "recipient_42" {
		t.Fatalf("hybrid recipient mapping was not preserved: %+v", got)
	}
	if got.ThreadTypeCode != 15 {
		t.Fatalf("expected thread type code 15, got %d", got.ThreadTypeCode)
	}
}

func TestExtractGroupThreadUsesKnownParticipantNamesWithoutSyntheticID(t *testing.T) {
	threadArgs := make([]any, 90)
	threadArgs[0] = "1793795130303"
	threadArgs[7] = "group_7001"
	threadArgs[9] = "2"
	threadArgs[10] = "inbox"

	innerPayload, _ := json.Marshal(map[string]any{
		"step": []any{
			[]any{5, "verifyContactRowExists", "member_a", "", "", "Ana"},
			[]any{5, "verifyContactRowExists", "member_b", "", "", "Bruno"},
			[]any{5, "addParticipantIdToGroupThread", "group_7001", "member_a"},
			[]any{5, "addParticipantIdToGroupThread", "group_7001", "member_b"},
			append([]any{5, "updateOrInsertThread"}, threadArgs...),
		},
	})
	outerPayload, _ := json.Marshal(map[string]any{
		"lightspeed_web_request": map[string]any{"payload": string(innerPayload)},
	})

	threads, _ := ExtractLightSpeedThreadsAndContacts("<script>" + string(outerPayload) + "</script>")
	if len(threads) != 1 {
		t.Fatalf("expected 1 thread, got %d", len(threads))
	}
	got := threads[0]
	if !got.IsGroup || got.Type != "group" || got.ThreadTypeCode != 2 {
		t.Fatalf("thread type 2 must remain a group: %+v", got)
	}
	if got.Name != "Ana, Bruno" {
		t.Fatalf("expected provider-backed participant names, got %q", got.Name)
	}
}
