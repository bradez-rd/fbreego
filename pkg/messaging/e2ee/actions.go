package e2ee

import (
	"context"
	"encoding/base64"
	"fmt"
)

// BridgeActions encapsula todas las acciones RPC soportadas por el protocolo E2EE de Messenger.
type BridgeActions struct {
	caller RPCCaller
}

// RPCCaller es la interfaz abstracta que ejecuta un método RPC sobre el protocolo E2EE.
type RPCCaller interface {
	Call(ctx context.Context, method string, params map[string]any) (map[string]any, error)
}

// NewBridgeActions crea una instancia de BridgeActions vinculada a un llamador RPC.
func NewBridgeActions(caller RPCCaller) *BridgeActions {
	return &BridgeActions{caller: caller}
}

// EditMessage edita el texto de un mensaje estándar.
func (b *BridgeActions) EditMessage(ctx context.Context, messageID, newText string) (map[string]any, error) {
	return b.caller.Call(ctx, "editMessage", map[string]any{
		"messageId": messageID,
		"newText":   newText,
	})
}

// UnsendMessage cancela el envío de un mensaje estándar.
func (b *BridgeActions) UnsendMessage(ctx context.Context, messageID string) (map[string]any, error) {
	return b.caller.Call(ctx, "unsendMessage", map[string]any{
		"messageId": messageID,
	})
}

// EditE2EEMessage edita el texto de un mensaje en una conversación cifrada E2EE.
func (b *BridgeActions) EditE2EEMessage(ctx context.Context, chatJID, messageID, newText string) (map[string]any, error) {
	return b.caller.Call(ctx, "editE2EEMessage", map[string]any{
		"chatJid":   chatJID,
		"messageId": messageID,
		"newText":   newText,
	})
}

// UnsendE2EEMessage cancela el envío de un mensaje cifrado E2EE.
func (b *BridgeActions) UnsendE2EEMessage(ctx context.Context, chatJID, messageID string) (map[string]any, error) {
	return b.caller.Call(ctx, "unsendE2EEMessage", map[string]any{
		"chatJid":   chatJID,
		"messageId": messageID,
	})
}

// SendTypingIndicator envía indicador de escritura a una conversación estándar.
func (b *BridgeActions) SendTypingIndicator(ctx context.Context, threadID int64, isTyping, isGroup bool) (map[string]any, error) {
	return b.caller.Call(ctx, "sendTypingIndicator", map[string]any{
		"threadId":   threadID,
		"isTyping":   isTyping,
		"isGroup":    isGroup,
		"threadType": 1,
	})
}

// SendE2EETyping envía indicador de escritura a una conversación cifrada.
func (b *BridgeActions) SendE2EETyping(ctx context.Context, chatJID string, isTyping bool) (map[string]any, error) {
	return b.caller.Call(ctx, "sendE2EETyping", map[string]any{
		"chatJid":  chatJID,
		"isTyping": isTyping,
	})
}

// MarkRead marca mensajes como leídos hasta una marca temporal dada.
func (b *BridgeActions) MarkRead(ctx context.Context, threadID, watermarkTs int64) (map[string]any, error) {
	return b.caller.Call(ctx, "markRead", map[string]any{
		"threadId":    threadID,
		"watermarkTs": watermarkTs,
	})
}

// SendE2EEAudio envía un clip de voz o audio cifrado de extremo a extremo.
func (b *BridgeActions) SendE2EEAudio(ctx context.Context, chatJID string, audioData []byte, mimeType string, duration int) (map[string]any, error) {
	if mimeType == "" {
		mimeType = "audio/ogg; codecs=opus"
	}
	return b.caller.Call(ctx, "sendE2EEAudio", map[string]any{
		"chatJid":  chatJID,
		"data":     base64.StdEncoding.EncodeToString(audioData),
		"mimeType": mimeType,
		"duration": duration,
		"ptt":      true,
	})
}

// SendE2EEImage envía una imagen cifrada de extremo a extremo.
func (b *BridgeActions) SendE2EEImage(ctx context.Context, chatJID string, imageData []byte, mimeType, caption string, width, height int) (map[string]any, error) {
	if mimeType == "" {
		mimeType = "image/jpeg"
	}
	return b.caller.Call(ctx, "sendE2EEImage", map[string]any{
		"chatJid":  chatJID,
		"data":     base64.StdEncoding.EncodeToString(imageData),
		"mimeType": mimeType,
		"caption":  caption,
		"width":    width,
		"height":   height,
	})
}

// DownloadMedia descarga y decodifica un archivo multimedia de Messenger.
func (b *BridgeActions) DownloadMedia(ctx context.Context, url string) ([]byte, error) {
	res, err := b.caller.Call(ctx, "downloadMedia", map[string]any{
		"url": url,
	})
	if err != nil {
		return nil, err
	}
	dataStr, ok := res["data"].(string)
	if !ok {
		return nil, fmt.Errorf("respuesta de descarga sin datos base64 válidos")
	}
	return base64.StdEncoding.DecodeString(dataStr)
}
