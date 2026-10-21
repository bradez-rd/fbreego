package lightspeed

import (
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
