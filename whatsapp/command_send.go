package whatsapp

import (
	"context"
	"strings"

	waProto "go.mau.fi/whatsmeow/proto/waProto"
	"google.golang.org/protobuf/proto"
)

func (c *Client) handleSendCommand(args []string) error {
	if len(args) < 2 {
		c.Logger.Errorf("Usage: send <jid> <text>")
		return nil
	}
	recipient, ok := c.parseJID(args[0])
	if !ok {
		return nil
	}
	msg := &waProto.Message{Conversation: proto.String(strings.Join(args[1:], " "))}
	if recipient.Server == types.GroupServer {
		msg.MessageContextInfo = &waProto.MessageContextInfo{MessageSecret: random.Bytes(32)}
	}
	resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
	if err != nil {
		c.Logger.Errorf("Error sending message: %v", err)
	} else {
		c.Logger.Infof("Message sent (server timestamp: %s)", resp.Timestamp)
	}
	return nil
}

