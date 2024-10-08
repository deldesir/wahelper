package whatsapp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"wahelper/utils"
)

func (c *Client) handlePairPhoneCommand(args []string) error {
	if c.WAClient.IsLoggedIn() {
		c.Logger.Infof("Already logged in")
		return nil
	}

	qrChan, cancel := c.WAClient.GetQRChannel(context.Background())
	defer cancel()

	c.Logger.Infof("Connecting to WhatsApp...")
	err := c.WAClient.Connect()
	if err != nil {
		c.Logger.Errorf("Failed to connect: %v", err)
		return err
	}

	c.Logger.Infof("Please scan the QR code to login")
	for evt := range qrChan {
		if evt.Event == "code" {
			// Print QR code to terminal
			fmt.Printf("QR Code: %s\n", evt.Code)
			// You can use a library like "github.com/mdp/qrterminal/v3" to display the QR code in the terminal
			// qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
		} else {
			c.Logger.Infof("Login event: %s", evt.Event)
		}
	}

	return nil
}

func (c *Client) handleLogoutCommand(args []string) error {
	err := c.WAClient.Logout()
	if err != nil {
		c.Logger.Errorf("Error logging out: %v", err)
	} else {
		c.Logger.Infof("Successfully logged out")
	}
	return err
}

func (c *Client) handleSetPushNameCommand(args []string) error {
	if len(args) == 0 {
		c.Logger.Errorf("Usage: setpushname <name>")
		return nil
	}
	pushName := strings.Join(args, " ")
	err := c.WAClient.SendAppState(appstate.BuildSettingPushName(pushName))
	if err != nil {
		c.Logger.Errorf("Error setting push name: %v", err)
	} else {
		c.Logger.Infof("Push name updated")
	}
	return err
}

func (c *Client) handleSetStatusCommand(args []string) error {
	if len(args) == 0 {
		c.Logger.Errorf("Usage: setstatus <message>")
		return nil
	}
	statusMessage := strings.Join(args, " ")
	err := c.WAClient.SetStatusMessage(statusMessage)
	if err != nil {
		c.Logger.Errorf("Error setting status message: %v", err)
	} else {
		c.Logger.Infof("Status updated")
	}
	return err
}

func (c *Client) handlePrivacySettingsCommand(args []string) error {
	resp, err := c.WAClient.TryFetchPrivacySettings(false)
	if err != nil {
		c.Logger.Errorf("Error fetching privacy settings: %v", err)
	} else {
		c.Logger.Infof("Privacy settings: %+v", resp)
	}
	return err
}

func (c *Client) handleSetPrivacySettingCommand(args []string) error {
	if len(args) < 2 {
		c.Logger.Errorf("Usage: setprivacysetting <setting> <value>")
		return nil
	}
	setting := types.PrivacySettingType(args[0])
	value := types.PrivacySetting(args[1])
	resp, err := c.WAClient.SetPrivacySetting(setting, value)
	if err != nil {
		c.Logger.Errorf("Error setting privacy setting: %v", err)
	} else {
		c.Logger.Infof("Privacy setting updated: %+v", resp)
	}
	return err
}

func (c *Client) handleGetStatusPrivacyCommand(args []string) error {
	resp, err := c.WAClient.GetStatusPrivacy()
	if err != nil {
		c.Logger.Errorf("Error getting status privacy: %v", err)
	} else {
		c.Logger.Infof("Status privacy: %+v", resp)
	}
	return err
}

func (c *Client) handleSetDisappearTimerCommand(args []string) error {
	if len(args) < 2 {
		c.Logger.Errorf("Usage: setdisappeartimer <jid> <days>")
		return nil
	}
	days, err := strconv.Atoi(args[1])
	if err != nil {
		c.Logger.Errorf("Invalid duration: %v", err)
		return nil
	}
	recipient, ok := utils.ParseJID(args[0])
	if !ok {
		c.Logger.Errorf("Invalid JID: %s", args[0])
		return nil
	}
	duration := time.Duration(days) * 24 * time.Hour
	err = c.WAClient.SetDisappearingTimer(recipient, duration)
	if err != nil {
		c.Logger.Errorf("Failed to set disappearing timer: %v", err)
	} else {
		c.Logger.Infof("Disappearing timer set for %s to %d days", recipient.String(), days)
	}
	return err
}

func (c *Client) handleSetDefaultDisappearTimerCommand(args []string) error {
	if len(args) < 1 {
		c.Logger.Errorf("Usage: setdefaultdisappeartimer <days>")
		return nil
	}
	days, err := strconv.Atoi(args[0])
	if err != nil {
		c.Logger.Errorf("Invalid duration: %v", err)
		return nil
	}
	duration := time.Duration(days) * 24 * time.Hour
	err = c.WAClient.SetDefaultDisappearingTimer(duration)
	if err != nil {
		c.Logger.Errorf("Failed to set default disappearing timer: %v", err)
	} else {
		c.Logger.Infof("Default disappearing timer set to %d days", days)
	}
	return err
}

func (c *Client) handleGetBlockListCommand(args []string) error {
	blocklist, err := c.WAClient.GetBlocklist()
	if err != nil {
		c.Logger.Errorf("Failed to get blocked contacts list: %v", err)
	} else {
		c.Logger.Infof("Blocklist: %+v", blocklist)
	}
	return err
}

func (c *Client) handleBlockCommand(args []string) error {
	if len(args) < 1 {
		c.Logger.Errorf("Usage: block <jid>")
		return nil
	}
	jid, ok := utils.ParseJID(args[0])
	if !ok {
		c.Logger.Errorf("Invalid JID: %s", args[0])
		return nil
	}
	resp, err := c.WAClient.UpdateBlocklist(jid, events.BlocklistChangeActionBlock)
	if err != nil {
		c.Logger.Errorf("Error updating blocklist: %v", err)
	} else {
		c.Logger.Infof("Blocked %s: %+v", jid.String(), resp)
	}
	return err
}

func (c *Client) handleUnblockCommand(args []string) error {
	if len(args) < 1 {
		c.Logger.Errorf("Usage: unblock <jid>")
		return nil
	}
	jid, ok := utils.ParseJID(args[0])
	if !ok {
		c.Logger.Errorf("Invalid JID: %s", args[0])
		return nil
	}
	resp, err := c.WAClient.UpdateBlocklist(jid, events.BlocklistChangeActionUnblock)
	if err != nil {
		c.Logger.Errorf("Error updating blocklist: %v", err)
	} else {
		c.Logger.Infof("Unblocked %s: %+v", jid.String(), resp)
	}
	return err
}
