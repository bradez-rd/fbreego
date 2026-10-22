package attachments

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/bradez-rd/fbreego/pkg/core/contracts"
	"github.com/bradez-rd/fbreego/pkg/core/httputil"
	"github.com/bradez-rd/fbreego/pkg/core/types"
	"github.com/bradez-rd/fbreego/pkg/core/utils"
)

// UploadURL devuelve la URL canónica de carga de adjuntos de Mercury.
func UploadURL() string {
	return contracts.MercuryUploadURL()
}

// Upload envía un archivo binario al endpoint de carga de Mercury de Facebook y devuelve el ID del adjunto.
func Upload(ctx context.Context, client *http.Client, fb *types.DataFB, filename string, r io.Reader, isVoiceClip bool) (*types.AttachmentResult, error) {
	if client == nil {
		client = http.DefaultClient
	}

	var bodyBuf bytes.Buffer
	writer := multipart.NewWriter(&bodyBuf)

	part, err := writer.CreateFormFile("upload_1024", filename)
	if err != nil {
		return nil, fmt.Errorf("error al crear form-data de adjunto: %w", err)
	}
	if _, err := io.Copy(part, r); err != nil {
		return nil, fmt.Errorf("error al copiar contenido del archivo: %w", err)
	}

	_ = writer.WriteField("fb_dtsg", fb.FbDtsg)
	_ = writer.WriteField("jazoest", fb.Jazoest)
	_ = writer.WriteField("__user", fb.FacebookID)
	_ = writer.WriteField("__a", "1")
	_ = writer.WriteField("__req", utils.NextRequestBase36())
	_ = writer.WriteField("__rev", fb.ClientRevision)
	if isVoiceClip {
		_ = writer.WriteField("voice_clip", "true")
	} else {
		_ = writer.WriteField("voice_clip", "false")
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", contracts.MercuryUploadURL(), &bodyBuf)
	if err != nil {
		return nil, err
	}

	headers := utils.DefaultHeaders("upload.facebook.com")
	for k, v := range headers {
		req.Header[k] = v
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Cookie", fb.CookieFacebook)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fallo al subir adjunto a facebook: %w", err)
	}
	defer resp.Body.Close()

	if err := httputil.CheckHTTPResponse(resp); err != nil {
		return nil, err
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	parsed, err := utils.ParseFacebookJSON(respBytes, true)
	if err != nil {
		return nil, err
	}

	payload, ok := parsed["payload"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("respuesta de carga sin payload válido")
	}
	metadataList, ok := payload["metadata"].([]any)
	if !ok || len(metadataList) == 0 {
		return nil, fmt.Errorf("respuesta de carga sin metadata de adjuntos")
	}

	meta, ok := metadataList[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("formato de metadata de adjunto inesperado")
	}

	result := &types.AttachmentResult{
		AttachmentDataSend: meta,
	}

	// Identificar tipo y ID
	if imgID, ok := meta["image_id"]; ok {
		result.AttachmentID = fmt.Sprintf("%v", imgID)
		result.AttachmentType = "image"
	} else if vidID, ok := meta["video_id"]; ok {
		result.AttachmentID = fmt.Sprintf("%v", vidID)
		result.AttachmentType = "video"
	} else if audID, ok := meta["audio_id"]; ok {
		result.AttachmentID = fmt.Sprintf("%v", audID)
		result.AttachmentType = "audio"
	} else if fileID, ok := meta["file_id"]; ok {
		result.AttachmentID = fmt.Sprintf("%v", fileID)
		result.AttachmentType = "file"
	}

	return result, nil
}
