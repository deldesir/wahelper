package whatsapp

func (c *Client) handlePairPhoneCommand(args []string) error {
    if len(args) < 1 {
        c.logger.Errorf("Usage: pair-phone <number>")
        return nil
    }
    // ... implementation ...
    return nil
}

func (c *Client) handleLogoutCommand(args []string) error {
    err := c.cli.Logout()
		if err != nil {
			c.Logger.Errorf("Error logging out: %v", err)
		} else {
			c.Logger.Infof("Successfully logged out")
		}
}

func (c *Client) handleSetPushNameCommand(args []string) error {
    if len(args) == 0 {
        log.Errorf("Usage: setpushname <name>")
        return nil
    }
    err := c.cli.SendAppState(appstate.BuildSettingPushName(strings.Join(args, " ")))
    if err != nil {
        c.Logger.Errorf("Error setting push name: %v", err)
    } else {
        c.Logger.Infof("Push name updated")
    }
}

func (c *Client) handleSetStatusCommand(args []string) error {
    if len(args) == 0 {
        log.Errorf("Usage: setstatus <message>")
        return
    }
    err := cli.SetStatusMessage(strings.Join(args, " "))
    if err != nil {
        log.Errorf("Error setting status message: %v", err)
    } else {
        log.Infof("Status updated")
    }
}

func (c *Client) handlePrivacySettingsCommand(args []string) error {
    resp, err := cli.TryFetchPrivacySettings(false)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("%+v\n", resp)
		}
}

func (c *Client) handleSetPrivacySettingCommand(args []string) error {
    if len(args) < 2 {
        log.Errorf("Usage: setprivacysetting <setting> <value>")
        return
    }
    setting := types.PrivacySettingType(args[0])
    value := types.PrivacySetting(args[1])
    resp, err := cli.SetPrivacySetting(setting, value)
    if err != nil {
        fmt.Println(err)
    } else {
        fmt.Printf("%+v\n", resp)
    }
}

func (c *Client) handleGetStatusPrivacyCommand(args []string) error {
    resp, err := cli.GetStatusPrivacy()
		fmt.Println(err)
		fmt.Println(resp)
}

func (c *Client) handleSetDisappearTimerCommand(args []string) error {
    if len(args) < 2 {
        log.Errorf("Usage: setdisappeartimer <jid> <days>")
        return
    }
    days, err := strconv.Atoi(args[1])
    if err != nil {
        log.Errorf("Invalid duration: %v", err)
        return
    }
    recipient, ok := parseJID(args[0])
    if !ok {
        return
    }
    err = cli.SetDisappearingTimer(recipient, time.Duration(days)*24*time.Hour)
    if err != nil {
        log.Errorf("Failed to set disappearing timer: %v", err)
    }
}

func (c *Client) handleSetDefaultDisappearTimerCommand(args []string) error {
    if len(args) < 1 {
        log.Errorf("Usage: setdefaultdisappeartimer <days>")
        return
    }
    days, err := strconv.Atoi(args[0])
    if err != nil {
        log.Errorf("Invalid duration: %v", err)
        return
    }
    err = cli.SetDefaultDisappearingTimer(time.Duration(days) * 24 * time.Hour)
    if err != nil {
        log.Errorf("Failed to set default disappearing timer: %v", err)
    }
}

func (c *Client) handleGetBlockListCommand(args []string) error {
    blocklist, err := cli.GetBlocklist()
		if err != nil {
			log.Errorf("Failed to get blocked contacts list: %v", err)
		} else {
			log.Infof("Blocklist: %+v", blocklist)
		}
}

func (c *Client) handleBlockCommand(args []string) error {
    if len(args) < 1 {
        log.Errorf("Usage: block <jid>")
        return
    }
    jid, ok := parseJID(args[0])
    if !ok {
        return
    }
    resp, err := cli.UpdateBlocklist(jid, events.BlocklistChangeActionBlock)
    if err != nil {
        log.Errorf("Error updating blocklist: %v", err)
    } else {
        log.Infof("Blocklist updated: %+v", resp)
    }
}

func (c *Client) handleUnblockCommand(args []string) error {
    if len(args) < 1 {
        log.Errorf("Usage: unblock <jid>")
        return
    }
    jid, ok := parseJID(args[0])
    if !ok {
        return
    }
    resp, err := cli.UpdateBlocklist(jid, events.BlocklistChangeActionUnblock)
    if err != nil {
        log.Errorf("Error updating blocklist: %v", err)
    } else {
        log.Infof("Blocklist updated: %+v", resp)
    }
}

