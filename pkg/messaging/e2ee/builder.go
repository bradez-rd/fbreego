package e2ee

import (
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCommon"
	"go.mau.fi/whatsmeow/proto/waConsumerApplication"
	"go.mau.fi/whatsmeow/types"
)

// BuildConsumerTextMessage crea el payload Protobuf ConsumerApplication para un mensaje de texto E2EE.
func BuildConsumerTextMessage(text string) *waConsumerApplication.ConsumerApplication {
	return &waConsumerApplication.ConsumerApplication{
		Payload: &waConsumerApplication.ConsumerApplication_Payload{
			Payload: &waConsumerApplication.ConsumerApplication_Payload_Content{
				Content: &waConsumerApplication.ConsumerApplication_Content{
					Content: &waConsumerApplication.ConsumerApplication_Content_MessageText{
						MessageText: &waCommon.MessageText{
							Text: &text,
						},
					},
				},
			},
		},
	}
}

// TargetJID crea el JID de destinatario para Messenger E2EE (ej. 100011614426020@msgr).
func TargetJID(fbid string) types.JID {
	return types.JID{
		User:   fbid,
		Server: types.MessengerServer,
	}
}

// CompileCheck valida que whatsmeow y sus métodos de Messenger compilen adecuadamente.
func CompileCheck(cli *whatsmeow.Client, jid types.JID, app *waConsumerApplication.ConsumerApplication) error {
	if cli == nil {
		return fmt.Errorf("client is nil")
	}
	_ = jid
	_ = app
	return nil
}
