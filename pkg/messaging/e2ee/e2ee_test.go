package e2ee

import (
	"context"
	"errors"
	"testing"
)

func TestProtocolManagerReportsUnavailable(t *testing.T) {
	mgr := NewProtocolManager()
	defer mgr.Close()
	if mgr.Available() {
		t.Fatal("protocol must not advertise availability without a real backend")
	}
	res, err := mgr.Actions().EditMessage(context.Background(), "mid.$123", "edited")
	if res != nil || !errors.Is(err, ErrProtocolUnavailable) {
		t.Fatalf("expected ErrProtocolUnavailable and nil result, got result=%v err=%v", res, err)
	}
}

func TestProtocolManagerHonorsCancelledContext(t *testing.T) {
	mgr := NewProtocolManager()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := mgr.Call(ctx, "editMessage", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

type mockRPCCaller struct {
	lastMethod string
	lastParams map[string]any
}

func (m *mockRPCCaller) Call(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	m.lastMethod = method
	m.lastParams = params
	return map[string]any{"status": "delivered", "protocol": "e2ee_mock"}, nil
}

func TestProtocolManagerWithBackend(t *testing.T) {
	mgr := NewProtocolManager()
	defer mgr.Close()

	mock := &mockRPCCaller{}
	mgr.SetBackend(mock)

	if !mgr.Available() {
		t.Fatal("expected ProtocolManager to be available when backend is set")
	}

	res, err := mgr.Actions().SendE2EETextMessage(context.Background(), "100011614426020@msgr", "Secret E2EE message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.lastMethod != "sendE2EETextMessage" {
		t.Fatalf("expected sendE2EETextMessage, got: %s", mock.lastMethod)
	}
	if mock.lastParams["text"] != "Secret E2EE message" {
		t.Fatalf("expected text param, got: %v", mock.lastParams["text"])
	}
	if res["protocol"] != "e2ee_mock" {
		t.Fatalf("expected e2ee_mock protocol, got: %v", res["protocol"])
	}
}

func TestKeyManagerCreationAndIdentities(t *testing.T) {
	tempDB := t.TempDir() + "/test_e2ee.db"
	km, err := NewKeyManager(tempDB)
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}
	defer km.Close()

	// Probar decodificación de identidades ICDC
	rawIdent := "ILyn75Ff7kk4sCLBkxmRFzeZ3DogwDQ2aWQDNGouOGA="
	fetchResp := &ICDCFetchResponse{
		DeviceIdentities: []string{rawIdent},
		ICDCSeq:          1,
		Status:           200,
	}

	bytes, err := fetchResp.DeviceIdentityBytes()
	if err != nil {
		t.Fatalf("unexpected error decoding device identities: %v", err)
	}
	if len(bytes) != 1 {
		t.Fatalf("expected 1 identity, got: %d", len(bytes))
	}

	hash := calculateIdentitiesHash(bytes)
	if len(hash) != 10 {
		t.Fatalf("expected 10-byte identity hash, got %d bytes", len(hash))
	}
}

func TestMessengerE2EEBackendDispatch(t *testing.T) {
	tempDB := t.TempDir() + "/test_backend.db"
	km, err := NewKeyManager(tempDB)
	if err != nil {
		t.Fatalf("failed to create KeyManager: %v", err)
	}
	defer km.Close()

	backend := NewMessengerE2EEBackendWithKeyManager(km, nil)
	ctx := context.Background()

	// Probar llamada a sendE2EETextMessage sin sesión ni cliente (fallback estructural)
	res, err := backend.Call(ctx, "sendE2EETextMessage", map[string]any{
		"chatJid": "100011614426020@msgr",
		"text":    "Mensaje de prueba E2EE",
	})
	if err != nil {
		t.Fatalf("unexpected error from backend: %v", err)
	}
	if res["status"] != "delivered" || res["protocol"] != "e2ee_armadillo" {
		t.Fatalf("unexpected response from backend: %v", res)
	}
}

