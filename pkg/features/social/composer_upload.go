package social

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/httputil"
)

// ComposerUploadResult representa el resultado de una subida a través de react_composer.
type ComposerUploadResult struct {
	PhotoID  string `json:"photoId"`
	ImageSrc string `json:"imageSrc,omitempty"`
}

// UploadComposerPhoto sube una foto usando el endpoint react_composer/attachments/photo/upload de Marketplace.
func UploadComposerPhoto(ctx context.Context, client *http.Client, fb *types.DataFB, targetID, filename string, r io.Reader) (*ComposerUploadResult, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if fb == nil {
		return nil, fmt.Errorf("datos de sesión no proporcionados")
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("error al leer contenido multimedia: %w", err)
	}

	if targetID == "" {
		targetID = "1490734647688555"
	}
	if filename == "" {
		filename = "attachment.jpg"
	}

	width, height := 900, 1200
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		width, height = cfg.Width, cfg.Height
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	_ = writer.WriteField("fb_dtsg", fb.FbDtsg)
	_ = writer.WriteField("qn", "comet_marketplace_composer")
	_ = writer.WriteField("target_id", targetID)
	_ = writer.WriteField("source", "8")
	_ = writer.WriteField("profile_id", fb.FacebookID)
	_ = writer.WriteField("waterfallxapp", "comet")

	ext := strings.ToLower(filepath.Ext(filename))
	mimeType := "image/jpeg"
	if ext == ".png" {
		mimeType = "image/png"
	} else if ext == ".webp" {
		mimeType = "image/webp"
	}

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="farr"; filename="%s"`, filepath.Base(filename)))
	h.Set("Content-Type", mimeType)
	part, err := writer.CreatePart(h)
	if err != nil {
		return nil, fmt.Errorf("error al crear parte multipart: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("error al escribir datos de imagen: %w", err)
	}

	_ = writer.WriteField("upload_id", "1024")
	_ = writer.WriteField("js_resized", "true")
	_ = writer.WriteField("original_file_size", fmt.Sprintf("%d", len(data)))
	_ = writer.WriteField("original_width", fmt.Sprintf("%d", width))
	_ = writer.WriteField("original_height", fmt.Sprintf("%d", height))
	_ = writer.WriteField("upload_width", "undefined")
	_ = writer.WriteField("upload_height", "undefined")

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("error al cerrar multipart writer: %w", err)
	}

	uploadURL := fmt.Sprintf("https://upload.facebook.com/ajax/react_composer/attachments/photo/upload?av=%s&__user=%s&__a=1&fb_dtsg=%s&jazoest=%s",
		url.QueryEscape(fb.FacebookID),
		url.QueryEscape(fb.FacebookID),
		url.QueryEscape(fb.FbDtsg),
		url.QueryEscape(fb.Jazoest),
	)

	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, &buf)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Cookie", fb.CookieFacebook)
	req.Header.Set("Origin", "https://www.facebook.com")
	req.Header.Set("Referer", "https://www.facebook.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error de red al subir imagen de composer: %w", err)
	}
	defer resp.Body.Close()

	if err := httputil.CheckHTTPResponse(resp); err != nil {
		return nil, err
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error al leer respuesta de subida: %w", err)
	}

	bodyClean := strings.TrimPrefix(string(respBody), "for (;;);")
	var parsed map[string]any
	if err := json.Unmarshal([]byte(bodyClean), &parsed); err != nil {
		return nil, fmt.Errorf("respuesta inválida de uploader: %s", string(respBody))
	}

	payload, ok := parsed["payload"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("respuesta sin payload válido de uploader: %v", parsed)
	}

	photoID, _ := payload["photoID"].(string)
	if photoID == "" {
		if pid, exists := payload["fbid"]; exists {
			photoID = fmt.Sprintf("%v", pid)
		}
	}
	if photoID == "" {
		return nil, fmt.Errorf("no se encontró photoID en la respuesta del uploader: %v", payload)
	}

	imageSrc, _ := payload["imageSrc"].(string)
	return &ComposerUploadResult{
		PhotoID:  photoID,
		ImageSrc: imageSrc,
	}, nil
}
