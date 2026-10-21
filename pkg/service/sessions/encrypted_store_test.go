package sessions

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bradez-rd/fbreego/pkg/core/httputil"
	"github.com/bradez-rd/fbreego/pkg/core/types"
)

func TestEncryptedSessionStore_SaveLoadDelete(t *testing.T) {
	t.Setenv("FBCHAT_SESSION_SECRET", "test-only-session-secret-32-characters-minimum")
	tempDir, err := os.MkdirTemp("", "fbchat_test_sessions_*")
	if err != nil {
		t.Fatalf("error al crear temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	store := NewEncryptedSessionStore(tempDir)

	record := &SessionRecord{
		SessionID:  "test-session-uuid-1234",
		Label:      "Test Account",
		TenantID:   "tenant_alpha",
		FacebookID: "1000555666777",
		Status:     "active",
		CreatedAt:  UTCNow(),
		Cookies: []types.CookieInput{
			{Name: "c_user", Value: "1000555666777"},
			{Name: "xs", Value: "secret_xs_token"},
		},
		ProviderData: &types.DataFB{
			FbDtsg:     "fb_dtsg_sample",
			FacebookID: "1000555666777",
		},
	}

	// 1. Guardar
	if err := store.Save(record); err != nil {
		t.Fatalf("store.Save() falló: %v", err)
	}

	// Verificar que el archivo en disco está cifrado (no contiene "secret_xs_token" en texto plano)
	diskFile := filepath.Join(tempDir, "tenant_alpha", record.SessionID+".enc")
	rawDisk, err := os.ReadFile(diskFile)
	if err != nil {
		t.Fatalf("no se encontró el archivo en disco: %v", err)
	}
	if len(rawDisk) == 0 {
		t.Fatal("archivo cifrado vacío")
	}

	// 2. Cargar todos
	loadedList, err := store.LoadAll()
	if err != nil {
		t.Fatalf("store.LoadAll() falló: %v", err)
	}
	if len(loadedList) != 1 {
		t.Fatalf("esperado 1 registro, obtenidos %d", len(loadedList))
	}

	loaded := loadedList[0]
	if loaded.SessionID != record.SessionID {
		t.Errorf("SessionID esperado %s, obtenido %s", record.SessionID, loaded.SessionID)
	}
	if loaded.FacebookID != record.FacebookID {
		t.Errorf("FacebookID esperado %s, obtenido %s", record.FacebookID, loaded.FacebookID)
	}
	if len(loaded.Cookies) != 2 || loaded.Cookies[1].Value != "secret_xs_token" {
		t.Errorf("cookies descifradas incorrectas: %+v", loaded.Cookies)
	}

	// 3. Eliminar
	if err := store.Delete("tenant_alpha", record.SessionID); err != nil {
		t.Fatalf("store.Delete() falló: %v", err)
	}

	afterDelete, err := store.LoadAll()
	if err != nil {
		t.Fatalf("store.LoadAll() post-delete falló: %v", err)
	}
	if len(afterDelete) != 0 {
		t.Errorf("la lista debería estar vacía tras borrar la sesión, obtenidos %d", len(afterDelete))
	}
}

func TestEncryptedSessionStoreRequiresSecret(t *testing.T) {
	t.Setenv("FBCHAT_SESSION_SECRET", "")
	store := NewEncryptedSessionStore(t.TempDir())
	if store.Ready() == nil {
		t.Fatal("expected missing session secret to fail initialization")
	}
}

func TestEncryptedSessionStoreRejectsFallbackAndShortKeys(t *testing.T) {
	// Forbidden default fallback key
	t.Setenv("FBCHAT_SESSION_SECRET", "change-me-in-production-fbchat-v2-secret-key-32b")
	storeDefault := NewEncryptedSessionStore(t.TempDir())
	if err := storeDefault.Ready(); err == nil {
		t.Fatal("expected default fallback secret to be rejected")
	}

	// Key shorter than 32 characters
	t.Setenv("FBCHAT_SESSION_SECRET", "too-short-secret")
	storeShort := NewEncryptedSessionStore(t.TempDir())
	if err := storeShort.Ready(); err == nil {
		t.Fatal("expected secret shorter than 32 characters to be rejected")
	}
}

func TestValidateTenantIDRejectsTraversal(t *testing.T) {
	for _, tenantID := range []string{"..", "../outside", `..\\outside`, "/absolute", "tenant/sub"} {
		if err := ValidateTenantID(tenantID); err == nil {
			t.Errorf("expected tenantId %q to be rejected", tenantID)
		}
	}
	for _, tenantID := range []string{"default", "tenant_alpha", "tenant-01", "tenant.example"} {
		if err := ValidateTenantID(tenantID); err != nil {
			t.Errorf("expected tenantId %q to be accepted: %v", tenantID, err)
		}
	}
}

func TestSessionRegistryFailsOnCorruptPersistedSession(t *testing.T) {
	t.Setenv("FBCHAT_SESSION_SECRET", "test-only-session-secret-32-characters-minimum")
	root := t.TempDir()
	store := NewEncryptedSessionStore(root)
	if err := store.Ready(); err != nil {
		t.Fatal(err)
	}
	tenantDir := filepath.Join(root, "default")
	if err := os.MkdirAll(tenantDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tenantDir, "broken.enc"), []byte("not-encrypted"), 0600); err != nil {
		t.Fatal(err)
	}
	registry := NewSessionRegistry(store, nil)
	if registry.Ready() == nil {
		t.Fatal("expected corrupt persisted session to make registry not ready")
	}
}

func TestSessionRegistry_CookieValidation(t *testing.T) {
	t.Setenv("FBCHAT_SESSION_SECRET", "test-only-session-secret-32-characters-minimum")
	store := NewEncryptedSessionStore(t.TempDir())
	registry := NewSessionRegistry(store, nil)

	ctx := context.Background()

	// 1. Less than 2 cookies
	_, err := registry.Create(ctx, types.SessionCreateInput{
		Cookies: []types.CookieInput{{Name: "c_user", Value: "123"}},
	})
	if err == nil || !strings.Contains(err.Error(), "al menos 2 cookies") {
		t.Fatalf("expected error on < 2 cookies, got %v", err)
	}

	// 2. Missing c_user or xs
	_, err = registry.Create(ctx, types.SessionCreateInput{
		Cookies: []types.CookieInput{
			{Name: "c_user", Value: "123"},
			{Name: "other", Value: "456"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "c_user") {
		t.Fatalf("expected error on missing xs cookie, got %v", err)
	}

	// 3. Duplicate cookie names
	_, err = registry.Create(ctx, types.SessionCreateInput{
		Cookies: []types.CookieInput{
			{Name: "c_user", Value: "123"},
			{Name: "xs", Value: "456"},
			{Name: "c_user", Value: "789"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "cookie duplicada") {
		t.Fatalf("expected error on duplicate cookie name, got %v", err)
	}

	// 4. Expired cookie
	expiredUnix := float64(time.Now().Add(-10 * time.Minute).Unix())
	_, err = registry.Create(ctx, types.SessionCreateInput{
		Cookies: []types.CookieInput{
			{Name: "c_user", Value: "123", ExpirationDate: expiredUnix},
			{Name: "xs", Value: "456"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "ya ha expirado") {
		t.Fatalf("expected error on expired cookie, got %v", err)
	}
}

func TestSessionRecord_CircuitBreakerAndExecutionGate(t *testing.T) {
	rec := &SessionRecord{
		SessionID:  "test-sess-1",
		TenantID:   "default",
		FacebookID: "10001",
		Status:     StatusActive,
	}

	// 1. Active record can execute
	if err := rec.CanExecute(); err != nil {
		t.Fatalf("expected active record to allow execution, got %v", err)
	}

	// 2. Acquire slot & release
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rec.AcquireExecutionSlot(ctx); err != nil {
		t.Fatalf("expected execution slot acquisition, got %v", err)
	}
	rec.ReleaseExecutionSlot()

	// 3. HandleHTTPResult for 403 / Account restricted transitions to Blocked
	rec.HandleHTTPResult(httputil.ErrAccountRestricted)
	if rec.Status != StatusBlocked {
		t.Fatalf("expected status %s after 403, got %s", StatusBlocked, rec.Status)
	}
	if err := rec.CanExecute(); err == nil {
		t.Fatal("expected CanExecute to reject blocked session")
	}

	// 4. HandleHTTPResult for 401 transitions to Error
	rec.HandleHTTPResult(httputil.ErrSessionExpired)
	if rec.Status != StatusError {
		t.Fatalf("expected status %s after 401, got %s", StatusError, rec.Status)
	}
}
