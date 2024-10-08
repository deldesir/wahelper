package whatsapp

import (
	"strings"

	"go.mau.fi/whatsmeow"
	"wahelper/utils"
)

func (c *Client) handleMediaConnCommand(args []string) error {
	conn, err := c.WAClient.DangerousInternals().RefreshMediaConn(false)
	if err != nil {
		c.Logger.Errorf("Failed to get media connection: %v", err)
	} else {
		c.Logger.Infof("Media connection: %+v", conn)
	}
	return err
}

func (c *Client) handleGetAvatarCommand(args []string) error {
	if len(args) < 1 {
		c.Logger.Errorf("Usage: getavatar <jid> [existing ID] [--preview] [--community]")
		return nil
	}
	jid, ok := utils.ParseJID(args[0])
	if !ok {
		c.Logger.Errorf("Invalid JID: %s", args[0])
		return nil
	}
	existingID := ""
	var preview, isCommunity bool
	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "--") {
			switch arg {
			case "--preview":
				preview = true
			case "--community":
				isCommunity = true
			default:
				c.Logger.Errorf("Unknown flag: %s", arg)
				return nil
			}
		} else {
			if existingID == "" {
				existingID = arg
			} else {
				c.Logger.Errorf("Unexpected argument: %s", arg)
				return nil
			}
		}
	}
	pic, err := c.WAClient.GetProfilePictureInfo(jid, &whatsmeow.GetProfilePictureParams{
		Preview:     preview,
		IsCommunity: isCommunity,
		ExistingID:  existingID,
	})
	if err != nil {
		c.Logger.Errorf("Failed to get avatar: %v", err)
		return err
	} else if pic != nil {
		c.Logger.Infof("Got avatar ID %s: %s", pic.ID, pic.URL)
	} else {
		c.Logger.Infof("No avatar found")
	}
	return nil
}
