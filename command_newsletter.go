package whatsapp

import (
	"context"
	"strconv"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"wahelper/utils"
)

func (c *Client) handleListNewslettersCommand(args []string) error {
	newsletters, err := c.WAClient.GetSubscribedNewsletters()
	if err != nil {
		c.Logger.Errorf("Failed to get subscribed newsletters: %v", err)
		return err
	}
	for _, newsletter := range newsletters {
		c.Logger.Infof("* %s: %s", newsletter.ID, newsletter.ThreadMeta.Name.Text)
	}
	return nil
}

func (c *Client) handleGetNewsletterCommand(args []string) error {
	if len(args) < 1 {
		c.Logger.Errorf("Usage: getnewsletter <jid>")
		return nil
	}
	jid, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	}
	meta, err := c.WAClient.GetNewsletterInfo(jid)
	if err != nil {
		c.Logger.Errorf("Failed to get info: %v", err)
	} else {
		c.Logger.Infof("Got info: %+v", meta)
	}
	return err
}

func (c *Client) handleGetNewsletterInviteCommand(args []string) error {
	if len(args) < 1 {
		c.Logger.Errorf("Usage: getnewsletterinvite <link>")
		return nil
	}
	meta, err := c.WAClient.GetNewsletterInfoWithInvite(args[0])
	if err != nil {
		c.Logger.Errorf("Failed to get info: %v", err)
	} else {
		c.Logger.Infof("Got info: %+v", meta)
	}
	return err
}

func (c *Client) handleLiveSubscribeNewsletterCommand(args []string) error {
	if len(args) < 1 {
		c.Logger.Errorf("Usage: livesubscribenewsletter <jid>")
		return nil
	}
	jid, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	}
	dur, err := c.WAClient.NewsletterSubscribeLiveUpdates(context.TODO(), jid)
	if err != nil {
		c.Logger.Errorf("Failed to subscribe to live updates: %v", err)
	} else {
		c.Logger.Infof("Subscribed to live updates for %s for %s", jid, dur)
	}
	return err
}

func (c *Client) handleGetNewsletterMessagesCommand(args []string) error {
	if len(args) < 1 {
		c.Logger.Errorf("Usage: getnewslettermessages <jid> [count] [before id]")
		return nil
	}
	jid, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	}
	count := 100
	if len(args) > 1 {
		var err error
		count, err = strconv.Atoi(args[1])
		if err != nil {
			c.Logger.Errorf("Invalid count: %v", err)
			return nil
		}
	}
	var before *types.MessageServerID
	if len(args) > 2 {
		beforeID := args[2]
		before = &beforeID
	}
	messages, err := c.WAClient.GetNewsletterMessages(jid, &whatsmeow.GetNewsletterMessagesParams{Count: count, Before: before})
	if err != nil {
		c.Logger.Errorf("Failed to get messages: %v", err)
	} else {
		for _, msg := range messages {
			c.Logger.Infof("%s: %+v (viewed %d times)", msg.MessageServerID, msg.Message, msg.ViewsCount)
		}
	}
	return err
}

func (c *Client) handleCreateNewsletterCommand(args []string) error {
	if len(args) < 1 {
		c.Logger.Errorf("Usage: createnewsletter <name>")
		return nil
	}
	resp, err := c.WAClient.CreateNewsletter(whatsmeow.CreateNewsletterParams{
		Name: strings.Join(args, " "),
	})
	if err != nil {
		c.Logger.Errorf("Failed to create newsletter: %v", err)
	} else {
		c.Logger.Infof("Created newsletter %+v", resp)
	}
	return err
}
