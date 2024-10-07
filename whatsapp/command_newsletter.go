package whatsapp

func (c *Client) handleListNewslettersCommand(args []string) error {
    newsletters, err := c.cli.GetSubscribedNewsletters()
    if err != nil {
        c.logger.Errorf("Failed to get subscribed newsletters: %v", err)
        return nil
    }
    for _, newsletter := range newsletters {
        c.logger.Infof("* %s: %s", newsletter.ID, newsletter.ThreadMeta.Name.Text)
    }
    return nil
}

func (c *Client) handleGetNewsletterCommand(args []string) error {
    jid, ok := parseJID(args[0])
		if !ok {
			return
		}
		meta, err := cli.GetNewsletterInfo(jid)
		if err != nil {
			log.Errorf("Failed to get info: %v", err)
		} else {
			log.Infof("Got info: %+v", meta)
		}
    }

func (c *Client) handleGetNewsletterInviteCommand(args []string) error {
    meta, err := cli.GetNewsletterInfoWithInvite(args[0])
		if err != nil {
			log.Errorf("Failed to get info: %v", err)
		} else {
			log.Infof("Got info: %+v", meta)
		}
    }

func (c *Client) handleLiveSubscribeNewsletterCommand(args []string) error {
    if len(args) < 1 {
        log.Errorf("Usage: livesubscribenewsletter <jid>")
        return
    }
    jid, ok := parseJID(args[0])
    if !ok {
        return
    }
    dur, err := cli.NewsletterSubscribeLiveUpdates(context.TODO(), jid)
    if err != nil {
        log.Errorf("Failed to subscribe to live updates: %v", err)
    } else {
        log.Infof("Subscribed to live updates for %s for %s", jid, dur)
    }
}

func (c *Client) handleGetNewsletterMessagesCommand(args []string) error {
    if len(args) < 1 {
        log.Errorf("Usage: getnewslettermessages <jid> [count] [before id]")
        return
    }
    jid, ok := parseJID(args[0])
    if !ok {
        return
    }
    count := 100
    var err error
    if len(args) > 1 {
        count, err = strconv.Atoi(args[1])
        if err != nil {
            log.Errorf("Invalid count: %v", err)
            return
        }
    }
    var before types.MessageServerID
    if len(args) > 2 {
        before, err = strconv.Atoi(args[2])
        if err != nil {
            log.Errorf("Invalid message ID: %v", err)
            return
        }
    }
    messages, err := cli.GetNewsletterMessages(jid, &whatsmeow.GetNewsletterMessagesParams{Count: count, Before: before})
    if err != nil {
        log.Errorf("Failed to get messages: %v", err)
    } else {
        for _, msg := range messages {
            log.Infof("%d: %+v (viewed %d times)", msg.MessageServerID, msg.Message, msg.ViewsCount)
        }
    }
}

func (c *Client) handleCreateNewsletterCommand(args []string) error {
    if len(args) < 1 {
        log.Errorf("Usage: createnewsletter <name>")
        return
    }
    resp, err := cli.CreateNewsletter(whatsmeow.CreateNewsletterParams{
        Name: strings.Join(args, " "),
    })
    if err != nil {
        log.Errorf("Failed to create newsletter: %v", err)
    } else {
        log.Infof("Created newsletter %+v", resp)
    }
}


