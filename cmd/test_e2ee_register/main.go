package main

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
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	gorillaWS "github.com/gorilla/websocket"
	"golang.org/x/crypto/curve25519"
	"go.mau.fi/libsignal/ecc"
	"go.mau.fi/whatsmeow/proto/waAdv"
	"go.mau.fi/whatsmeow/proto/waArmadilloICDC"
	"go.mau.fi/whatsmeow/proto/waWa6"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"

	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

var cookiesAccountB = `datr=4xHcap9NGNgD_8CvAl604o9t; fr=1IsbYI7EzBKa7I0su.AWdvwVPOCwL48AT50WLmMFm___u6QG2AcXOs9pOFYsZ8G0iphdg.Bq3EYb..AAA.0.0.Bq3EYb.AWce1zIdytJ_TvtCGvYpqvywXnQ; xs=32%3ATI9qwv8V9gvUpw%3A2%3A1792807619%3A-1%3A-1%3A%3AAcxOYoOVorDfNEbfzCk4PIvazDaov35cNgOLyEYwVA; c_user=100011614426020; sb=5xHcap2NrHOWP4M-BlvEuPq_`

type MessengerWebInitData struct {
	AccountKey      string `json:"accountKey"`
	AccountKeyV2    string `json:"accountKeyV2"`
	AppID           int64  `json:"appId"`
	CryptoAuthToken struct {
		EncryptedSerializedCat  string `json:"encrypted_serialized_cat"`
		ExpirationTimeInSeconds int64  `json:"expiration_time_in_seconds"`
	} `json:"cryptoAuthToken"`
	SessionID string      `json:"sessionId"`
	UserID    json.Number `json:"userId"`
}

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
			err = fmt.Errorf("device identity is %d bytes, expected 32", len(ident))
			break
		}
		deviceIdentityBytes[i] = [32]byte(ident)
	}
	return
}

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

func doE2EERequest(ctx context.Context, client *http.Client, endpoint string, body url.Values, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return fmt.Errorf("failed to prepare request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("accept-language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("sec-fetch-dest", "empty")
	req.Header.Set("sec-fetch-mode", "cors")
	req.Header.Set("sec-fetch-site", "cross-site")
	req.Header.Set("origin", "https://www.facebook.com")
	req.Header.Set("referer", "https://www.facebook.com/messages/t/")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("   [ICDC %s] Status: %d, Response: %s\n", endpoint, resp.StatusCode, string(respBody))
	if resp.StatusCode >= 300 || resp.StatusCode < 200 {
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(respBody))
	}
	return json.Unmarshal(respBody, into)
}

func main() {
	fmt.Println("==========================================================")
	fmt.Println("🔐 REGISTRO DE LLAVES E2EE ARMADILLO (ICDC + WHATSMEOW)")
	fmt.Println("==========================================================")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	httpClient := &http.Client{Timeout: 45 * time.Second}

	// 1. Extraer MessengerWebInitData
	targetURL := "https://www.facebook.com/messages/e2ee/t/1305701081633035/"
	req, _ := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	headers := utils.DefaultHeaders("www.facebook.com")
	for k, v := range headers {
		req.Header[k] = v
	}
	req.Header.Set("Cookie", cookiesAccountB)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")

	fmt.Println("1. Obteniendo credenciales E2EE desde Facebook HTML...")
	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	html := string(bodyBytes)

	reInit := regexp.MustCompile(`\["MessengerWebInitData",\[\],({.*?})\s*,\s*\d+\]`)
	matchInit := reInit.FindStringSubmatch(html)
	if len(matchInit) < 2 {
		fmt.Println("❌ No se encontró MessengerWebInitData en el HTML")
		return
	}

	var initData MessengerWebInitData
	if err := json.Unmarshal([]byte(matchInit[1]), &initData); err != nil {
		fmt.Printf("❌ Error decodificando MessengerWebInitData: %v\n", err)
		return
	}

	fbid, _ := strconv.ParseInt(initData.UserID.String(), 10, 64)
	fmt.Printf("✅ Extraído: UserID=%d, AppID=%d, Expira=%s\n",
		fbid, initData.AppID, time.Unix(initData.CryptoAuthToken.ExpirationTimeInSeconds, 0).Format(time.RFC3339))

	// 2. Inicializar SQLite Store y Whatsmeow Device
	dbPath := "file:e2ee_test_store.db?_pragma=foreign_keys(1)"
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Printf("Error abriendo DB sqlite: %v\n", err)
		return
	}
	defer sqlDB.Close()

	container := sqlstore.NewWithDB(sqlDB, "sqlite3", waLog.Noop)
	err = container.Upgrade(ctx)
	if err != nil {
		fmt.Printf("Error actualizando schema sqlstore: %v\n", err)
		return
	}

	var device *store.Device
	devices, err := container.GetAllDevices(ctx)
	if err == nil && len(devices) > 0 {
		device = devices[0]
		fmt.Printf("✅ Dispositivo existente encontrado en DB: JID=%s, UUID=%s\n", device.ID, device.FacebookUUID)
	}

	if device == nil {
		device = container.NewDevice()
		device.FacebookUUID = uuid.New()
		fmt.Printf("✅ Nuevo dispositivo generado: UUID=%s\n", device.FacebookUUID)
	}

	// 3. ICDC Fetch
	fmt.Println("2. Consultando identidades existentes con ICDC Fetch...")
	fetchBody := url.Values{
		"fbid":      {strconv.FormatInt(fbid, 10)},
		"fb_cat":    {initData.CryptoAuthToken.EncryptedSerializedCat},
		"app_id":    {strconv.FormatInt(initData.AppID, 10)},
		"device_id": {device.FacebookUUID.String()},
	}

	var fetchResp ICDCFetchResponse
	err = doE2EERequest(ctx, httpClient, "https://reg-e2ee.facebook.com/v2/fb_icdc_fetch", fetchBody, &fetchResp)
	if err != nil {
		fmt.Printf("❌ Error en ICDC Fetch: %v\n", err)
		return
	}
	fmt.Printf("✅ ICDC Fetch exitoso: Seq=%d, Dispositivos=%d\n", fetchResp.ICDCSeq, len(fetchResp.DeviceIdentities))

	// 4. Preparar lista de identidades y firma
	deviceIdentities, err := fetchResp.DeviceIdentityBytes()
	if err != nil {
		fmt.Printf("❌ Error decodificando identidades de dispositivos: %v\n", err)
		return
	}

	ownIdentityIndex := slices.Index(deviceIdentities, *device.IdentityKey.Pub)
	if ownIdentityIndex == -1 {
		ownIdentityIndex = len(deviceIdentities)
		deviceIdentities = append(deviceIdentities, *device.IdentityKey.Pub)
		fetchResp.ICDCSeq++
	}

	icdcTS := time.Now().Unix()
	unsignedList, err := proto.Marshal(&waArmadilloICDC.ICDCIdentityList{
		Seq:                proto.Int32(int32(fetchResp.ICDCSeq)),
		Timestamp:          proto.Int64(icdcTS),
		Devices:            sliceifyIdentities(deviceIdentities),
		SigningDeviceIndex: proto.Int32(int32(ownIdentityIndex)),
	})
	if err != nil {
		fmt.Printf("❌ Error serializando ICDCIdentityList: %v\n", err)
		return
	}

	signature := ecc.CalculateSignature(ecc.NewDjbECPrivateKey(*device.IdentityKey.Priv), unsignedList)
	signedList, err := proto.Marshal(&waArmadilloICDC.SignedICDCIdentityList{
		Details:   unsignedList,
		Signature: signature[:],
	})
	if err != nil {
		fmt.Printf("❌ Error serializando SignedICDCIdentityList: %v\n", err)
		return
	}

	// 5. ICDC Register
	fmt.Println("3. Registrando dispositivo en Meta ICDC (fb_register_v2)...")
	regBody := url.Values{
		"fbid":       {strconv.FormatInt(fbid, 10)},
		"fb_cat":     {initData.CryptoAuthToken.EncryptedSerializedCat},
		"app_id":     {strconv.FormatInt(initData.AppID, 10)},
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
	err = doE2EERequest(ctx, httpClient, "https://reg-e2ee.facebook.com/v2/fb_register_v2", regBody, &regResp)
	if err != nil {
		fmt.Printf("❌ Error en ICDC Register: %v\n", err)
		return
	}

	fmt.Printf("🎉 ¡REGISTRO E2EE EXITOSO! Status=%d, WA Device ID=%d, Type=%s\n", regResp.Status, regResp.WADeviceID, regResp.Type)

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

	err = device.Save(ctx)
	if err != nil {
		fmt.Printf("⚠️ Error guardando dispositivo: %v\n", err)
	} else {
		fmt.Printf("💾 Dispositivo guardado correctamente en SQLite con JID: %s\n", device.ID)
	}

	// 6. Conectar mediante websocket a la red E2EE de Messenger
	wsURL := fmt.Sprintf("wss://web-chat-e2ee.facebook.com/ws/chat?cid=client-%d", time.Now().UnixMilli())
	fmt.Printf("4. Conectando websocket a %s...\n", wsURL)

	dialer := gorillaWS.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	wsHeader := http.Header{
		"Origin":     {"https://www.facebook.com"},
		"User-Agent": {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"},
		"Cookie":     {cookiesAccountB},
	}

	rawConn, rawResp, err := dialer.DialContext(ctx, wsURL, wsHeader)
	if err != nil {
		if rawResp != nil {
			fmt.Printf("❌ Dial error (HTTP %d): %v\n", rawResp.StatusCode, err)
		} else {
			fmt.Printf("❌ Dial error: %v\n", err)
		}
		return
	}
	defer rawConn.Close()
	fmt.Printf("✅ WebSocket conectado exitosamente! Handshake HTTP %d\n", rawResp.StatusCode)

	// Enviar WA_HEADER + ClientHello
	var ephPriv [32]byte
	_, _ = io.ReadFull(strings.NewReader(uuid.New().String()+uuid.New().String()), ephPriv[:])
	ephPub, _ := curve25519.X25519(ephPriv[:], curve25519.Basepoint)

	clientHelloMsg, _ := proto.Marshal(&waWa6.HandshakeMessage{
		ClientHello: &waWa6.HandshakeMessage_ClientHello{
			Ephemeral: ephPub,
		},
	})

	// Header [W, A, 6, 3] + [len >> 16, len >> 8, len] + data
	frameLen := len(clientHelloMsg)
	header := []byte{'W', 'A', 6, 3, byte(frameLen >> 16), byte(frameLen >> 8), byte(frameLen)}
	fullHello := append(header, clientHelloMsg...)

	fmt.Printf("   Enviando ClientHello (%d bytes)...\n", len(fullHello))
	err = rawConn.WriteMessage(gorillaWS.BinaryMessage, fullHello)
	if err != nil {
		fmt.Printf("❌ Error enviando ClientHello: %v\n", err)
		return
	}

	// Leer respuesta ServerHello
	_ = rawConn.SetReadDeadline(time.Now().Add(10 * time.Second))
	msgType, respBytes, err := rawConn.ReadMessage()
	if err != nil {
		fmt.Printf("❌ Error leyendo ServerHello: %v\n", err)
		return
	}

	fmt.Printf("🎉 ¡RECIBIDA RESPUESTA DEL SERVIDOR! Tipo=%d, Longitud=%d bytes\n", msgType, len(respBytes))
	if len(respBytes) > 3 {
		// Descartar los 3 bytes de longitud si viene con framing
		payloadBytes := respBytes
		if len(respBytes) > 3 {
			payloadBytes = respBytes[3:]
		}
		var srvHello waWa6.HandshakeMessage
		if err := proto.Unmarshal(payloadBytes, &srvHello); err == nil && srvHello.ServerHello != nil {
			fmt.Printf("🎉 ¡SERVER HELLO VÁLIDO DE META! Ephemeral len=%d, Static len=%d, Cert len=%d\n",
				len(srvHello.ServerHello.Ephemeral),
				len(srvHello.ServerHello.Static),
				len(srvHello.ServerHello.Payload))
		} else {
			fmt.Printf("   Raw Server Response: %x\n", respBytes)
		}
	}
}
