package e2ee

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.mau.fi/libsignal/ecc"
	"go.mau.fi/whatsmeow/proto/waAdv"
	"go.mau.fi/whatsmeow/proto/waArmadilloICDC"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"

	coreTypes "github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

var (
	reCATRegex  = regexp.MustCompile(`"encrypted_serialized_cat":\s*"([^"]+)"`)
	reInitRegex = regexp.MustCompile(`\["MessengerWebInitData",\[\],({.*?})\s*,\s*\d+\]`)
)

// MessengerWebInitData contiene los metadatos de inicio de Messenger extraídos del HTML.
type MessengerWebInitData struct {
	AccountKey      string `json:"accountKey,omitempty"`
	AccountKeyV2    string `json:"accountKeyV2,omitempty"`
	AppID           int64  `json:"appId,omitempty"`
	CryptoAuthToken struct {
		EncryptedSerializedCat  string `json:"encrypted_serialized_cat,omitempty"`
		ExpirationTimeInSeconds int64  `json:"expiration_time_in_seconds,omitempty"`
	} `json:"cryptoAuthToken,omitempty"`
	SessionID string      `json:"sessionId,omitempty"`
	UserID    json.Number `json:"userId,omitempty"`
}

// ICDCFetchResponse es la respuesta de https://reg-e2ee.facebook.com/v2/fb_icdc_fetch.
type ICDCFetchResponse struct {
	DeviceIdentities []string `json:"device_identities"`
	ICDCSeq          int      `json:"icdc_seq"`
	Status           int      `json:"status"`
}

func (ifr *ICDCFetchResponse) DeviceIdentityBytes() (deviceIdentityBytes [][32]byte, err error) {
	deviceIdentityBytes = make([][32]byte, len(ifr.DeviceIdentities))
	for i, deviceIdentity := range ifr.DeviceIdentities {
		var ident []byte
		ident, err = base64.RawURLEncoding.DecodeString(deviceIdentity)
		if err != nil {
			ident, err = base64.StdEncoding.DecodeString(deviceIdentity)
		}
		if err != nil {
			break
		} else if len(ident) != 32 {
			err = fmt.Errorf("identidad de dispositivo tiene %d bytes, esperado 32", len(ident))
			break
		}
		deviceIdentityBytes[i] = [32]byte(ident)
	}
	return
}

// ICDCRegisterResponse es la respuesta de https://reg-e2ee.facebook.com/v2/fb_register_v2.
type ICDCRegisterResponse struct {
	ICDCSuccess bool   `json:"icdc_success"`
	Product     string `json:"product"`
	Status      int    `json:"status"`
	Type        string `json:"type"`
	WADeviceID  int    `json:"wa_device_id"`
}

func sliceifyIdentities(identities [][32]byte) [][]byte {
	sliceIdentities := make([][]byte, len(identities))
	for i, identity := range identities {
		sliceIdentities[i] = identity[:]
	}
	return sliceIdentities
}

func calculateIdentitiesHash(identities [][32]byte) []byte {
	identitiesSlice := sliceifyIdentities(identities)
	slices.SortFunc(identitiesSlice, bytes.Compare)
	ihash := sha256.Sum256(bytes.Join(identitiesSlice, nil))
	return ihash[:10]
}

// KeyManager administra las identidades criptográficas Curve25519 y registros ICDC para cuentas de Messenger.
type KeyManager struct {
	mu        sync.RWMutex
	dbPath    string
	sqlDB     *sql.DB
	container *sqlstore.Container
}

var defaultKeyManager *KeyManager
var defaultKeyManagerOnce sync.Once

// DefaultKeyManager devuelve una instancia singleton del gestor de llaves E2EE.
func DefaultKeyManager() (*KeyManager, error) {
	var err error
	defaultKeyManagerOnce.Do(func() {
		runtimeDir := ".runtime"
		_ = os.MkdirAll(runtimeDir, 0755)
		dbPath := filepath.Join(runtimeDir, "e2ee_keys.db")
		defaultKeyManager, err = NewKeyManager(dbPath)
	})
	return defaultKeyManager, err
}

// NewKeyManager inicializa un nuevo gestor de llaves respaldado por una base de datos SQLite.
func NewKeyManager(dbPath string) (*KeyManager, error) {
	connStr := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)", filepath.ToSlash(dbPath))
	sqlDB, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("error al abrir base de datos sqlite e2ee: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)

	container := sqlstore.NewWithDB(sqlDB, "sqlite3", waLog.Noop)
	if err := container.Upgrade(context.Background()); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("error actualizando esquema de base de datos e2ee: %w", err)
	}

	return &KeyManager{
		dbPath:    dbPath,
		sqlDB:     sqlDB,
		container: container,
	}, nil
}

// Close cierra la base de datos de llaves.
func (km *KeyManager) Close() error {
	km.mu.Lock()
	defer km.mu.Unlock()
	if km.sqlDB != nil {
		return km.sqlDB.Close()
	}
	return nil
}

// ExtractMessengerInitData extrae MessengerWebInitData desde el HTML de Messenger o la página principal.
func ExtractMessengerInitData(ctx context.Context, httpClient *http.Client, fb *coreTypes.DataFB, targetURL string) (*MessengerWebInitData, error) {
	if strings.TrimSpace(targetURL) == "" {
		targetURL = "https://www.facebook.com/messages/t/"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error preparando petición para init data: %w", err)
	}

	headers := utils.DefaultHeaders("www.facebook.com")
	for k, v := range headers {
		req.Header[k] = v
	}
	if fb != nil && fb.CookieFacebook != "" {
		req.Header.Set("Cookie", fb.CookieFacebook)
	}
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error realizando petición de init data: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error leyendo respuesta de init data: %w", err)
	}

	html := string(bodyBytes)
	matchInit := reInitRegex.FindStringSubmatch(html)
	if len(matchInit) > 1 {
		var initData MessengerWebInitData
		if err := json.Unmarshal([]byte(matchInit[1]), &initData); err == nil {
			return &initData, nil
		}
	}

	// Búsqueda por coincidencia directa de CAT si MessengerWebInitData no se serializó como bloque JSON estándar
	matchCAT := reCATRegex.FindStringSubmatch(html)
	if len(matchCAT) > 1 {
		initData := &MessengerWebInitData{}
		initData.CryptoAuthToken.EncryptedSerializedCat = matchCAT[1]
		initData.AppID = 2220391788200892
		if fb != nil && fb.FacebookID != "" {
			initData.UserID = json.Number(fb.FacebookID)
		}
		return initData, nil
	}

	return nil, fmt.Errorf("no se encontró MessengerWebInitData ni token CAT en el HTML")
}

// EnsureDevice asegura que la cuenta disponga de un dispositivo Curve25519 registrado ante Meta ICDC.
func (km *KeyManager) EnsureDevice(ctx context.Context, httpClient *http.Client, fb *coreTypes.DataFB) (*store.Device, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	if fb == nil {
		return nil, fmt.Errorf("datos de sesión DataFB son nulos")
	}

	var fbid int64
	if fb.FacebookID != "" {
		fbid, _ = strconv.ParseInt(fb.FacebookID, 10, 64)
	}

	// Buscar si ya existe un dispositivo registrado en SQLite
	devices, err := km.container.GetAllDevices(ctx)
	if err == nil {
		for _, dev := range devices {
			if dev.ID != nil && dev.ID.User == fb.FacebookID && dev.ID.Server == types.MessengerServer {
				return dev, nil
			}
		}
	}

	// Si no existe, crear un nuevo dispositivo local
	device := km.container.NewDevice()
	if fb.E2EEDeviceUUID != "" {
		if parsed, err := uuid.Parse(fb.E2EEDeviceUUID); err == nil {
			device.FacebookUUID = parsed
		}
	}
	if device.FacebookUUID == uuid.Nil {
		device.FacebookUUID = uuid.New()
		fb.E2EEDeviceUUID = device.FacebookUUID.String()
	}

	// Obtener o verificar CAT y AppID
	var cat string
	var appID int64 = 2220391788200892
	if fb.EncryptedSerializedCat != "" && fb.CatExpiration > time.Now().Unix() {
		cat = fb.EncryptedSerializedCat
		if fb.AppID != 0 {
			appID = fb.AppID
		}
	} else {
		initData, err := ExtractMessengerInitData(ctx, httpClient, fb, "")
		if err != nil {
			return nil, fmt.Errorf("fallo al extraer CAT para registro E2EE: %w", err)
		}
		cat = initData.CryptoAuthToken.EncryptedSerializedCat
		if initData.AppID != 0 {
			appID = initData.AppID
		}
		fb.EncryptedSerializedCat = cat
		fb.AppID = appID
		fb.CatExpiration = initData.CryptoAuthToken.ExpirationTimeInSeconds
	}

	if cat == "" {
		return nil, fmt.Errorf("el token CAT (CryptoAuthToken) está vacío")
	}

	// 1. ICDC Fetch
	fetchBody := url.Values{
		"fbid":      {strconv.FormatInt(fbid, 10)},
		"fb_cat":    {cat},
		"app_id":    {strconv.FormatInt(appID, 10)},
		"device_id": {device.FacebookUUID.String()},
	}

	var fetchResp ICDCFetchResponse
	if err := doICDCRequest(ctx, httpClient, "https://reg-e2ee.facebook.com/v2/fb_icdc_fetch", fetchBody, fb, &fetchResp); err != nil {
		return nil, fmt.Errorf("fallo en ICDC Fetch: %w", err)
	}

	deviceIdentities, err := fetchResp.DeviceIdentityBytes()
	if err != nil {
		return nil, fmt.Errorf("error decodificando identidades de dispositivos ICDC: %w", err)
	}

	ownIdentityIndex := slices.Index(deviceIdentities, *device.IdentityKey.Pub)
	if ownIdentityIndex == -1 {
		ownIdentityIndex = len(deviceIdentities)
		deviceIdentities = append(deviceIdentities, *device.IdentityKey.Pub)
		fetchResp.ICDCSeq++
	}

	// 2. Firmar ICDC Identity List
	icdcTS := time.Now().Unix()
	unsignedList, err := proto.Marshal(&waArmadilloICDC.ICDCIdentityList{
		Seq:                proto.Int32(int32(fetchResp.ICDCSeq)),
		Timestamp:          proto.Int64(icdcTS),
		Devices:            sliceifyIdentities(deviceIdentities),
		SigningDeviceIndex: proto.Int32(int32(ownIdentityIndex)),
	})
	if err != nil {
		return nil, fmt.Errorf("error serializando ICDCIdentityList: %w", err)
	}

	signature := ecc.CalculateSignature(ecc.NewDjbECPrivateKey(*device.IdentityKey.Priv), unsignedList)
	signedList, err := proto.Marshal(&waArmadilloICDC.SignedICDCIdentityList{
		Details:   unsignedList,
		Signature: signature[:],
	})
	if err != nil {
		return nil, fmt.Errorf("error serializando SignedICDCIdentityList: %w", err)
	}

	// 3. ICDC Register
	regBody := url.Values{
		"fbid":       {strconv.FormatInt(fbid, 10)},
		"fb_cat":     {cat},
		"app_id":     {strconv.FormatInt(appID, 10)},
		"device_id":  {device.FacebookUUID.String()},
		"e_regid":    {base64.StdEncoding.EncodeToString(binary.BigEndian.AppendUint32(nil, device.RegistrationID))},
		"e_keytype":  {base64.StdEncoding.EncodeToString([]byte{ecc.DjbType})},
		"e_ident":    {base64.StdEncoding.EncodeToString(device.IdentityKey.Pub[:])},
		"e_skey_id":  {base64.StdEncoding.EncodeToString(binary.BigEndian.AppendUint32(nil, device.SignedPreKey.KeyID)[1:])},
		"e_skey_val": {base64.StdEncoding.EncodeToString(device.SignedPreKey.Pub[:])},
		"e_skey_sig": {base64.StdEncoding.EncodeToString(device.SignedPreKey.Signature[:])},
		"icdc_list":  {base64.StdEncoding.EncodeToString(signedList)},
		"icdc_ts":    {strconv.FormatInt(icdcTS, 10)},
		"icdc_seq":   {strconv.Itoa(fetchResp.ICDCSeq)},
		"ihash":      {base64.StdEncoding.EncodeToString(calculateIdentitiesHash(deviceIdentities))},
	}

	var regResp ICDCRegisterResponse
	if err := doICDCRequest(ctx, httpClient, "https://reg-e2ee.facebook.com/v2/fb_register_v2", regBody, fb, &regResp); err != nil {
		return nil, fmt.Errorf("fallo en ICDC Register: %w", err)
	}

	if regResp.Status != 200 {
		return nil, fmt.Errorf("registro ICDC retornó código HTTP interno %d", regResp.Status)
	}

	device.ID = &types.JID{
		User:   strconv.FormatInt(fbid, 10),
		Device: uint16(regResp.WADeviceID),
		Server: types.MessengerServer,
	}
	device.Account = &waAdv.ADVSignedDeviceIdentity{
		Details:             make([]byte, 0),
		AccountSignatureKey: make([]byte, 32),
		AccountSignature:    make([]byte, 64),
		DeviceSignature:     make([]byte, 64),
	}

	if err := device.Save(ctx); err != nil {
		return nil, fmt.Errorf("fallo al guardar dispositivo en sqlite store: %w", err)
	}

	fb.WADeviceID = regResp.WADeviceID
	return device, nil
}

func doICDCRequest(ctx context.Context, client *http.Client, endpoint string, body url.Values, fb *coreTypes.DataFB, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return fmt.Errorf("error preparando petición ICDC: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("accept-language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "cross-site")
	req.Header.Set("origin", "https://www.facebook.com")
	req.Header.Set("referer", "https://www.facebook.com/messages/t/")
	if fb != nil && fb.CookieFacebook != "" {
		req.Header.Set("Cookie", fb.CookieFacebook)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error enviando petición ICDC: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error leyendo respuesta ICDC: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("respuesta ICDC con estado HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	return json.Unmarshal(respBytes, into)
}
