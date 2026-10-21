package benchmarks

import (
	"os"
	"testing"

	"github.com/bradez-rd/fbreego/pkg/core/logger"
	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
	"github.com/bradez-rd/fbreego/pkg/service/sessions"
)

func TestBenchmarksSmoke(t *testing.T) {
	// Smoke test to ensure benchmark functions run without panic
	_ = utils.GenThreadingID()
	_ = utils.NextRequestBase36()
	_, _ = utils.ParseFacebookJSON([]byte(`for (;;);{"status":"ok"}`), true)
}

func BenchmarkGenThreadingID(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = utils.GenThreadingID()
	}
}

func BenchmarkNextRequestBase36(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = utils.NextRequestBase36()
	}
}

func BenchmarkParseFacebookJSON(b *testing.B) {
	payload := []byte(`for (;;);{"payload":{"actions":[{"message_id":"mid.$123456789","status":"SUCCESS"}]}}`)
	b.ReportAllocs()
	for b.Loop() {
		_, _ = utils.ParseFacebookJSON(payload, true)
	}
}

func BenchmarkRedactSensitive(b *testing.B) {
	sample := `Header Cookie: c_user=100012345678; xs=secret_token_value_abc; datr=random_datr_123;`
	b.ReportAllocs()
	for b.Loop() {
		_ = logger.RedactSensitive(sample)
	}
}

func BenchmarkSessionStoreSaveAndLoad(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "fbreego_bench_*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store := sessions.NewEncryptedSessionStore(tmpDir)
	record := &sessions.SessionRecord{
		SessionID: "bench_session",
		TenantID:  "default",
		Cookies: []types.CookieInput{
			{Name: "c_user", Value: "100012345678"},
			{Name: "xs", Value: "test_token_secret"},
		},
		FacebookID: "100012345678",
	}

	b.ReportAllocs()
	for b.Loop() {
		_ = store.Save(record)
		_, _ = store.LoadAll()
	}
}
