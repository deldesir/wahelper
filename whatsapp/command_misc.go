package whatsapp

func (c *Client) handleReconnectCommand(args []string) error {
    c.isConnected = false
    c.cli.Disconnect()
    err := c.cli.Connect()
    if err != nil {
        c.logger.Errorf("Failed to reconnect: %v", err)
    }
    return nil
}

func (c *Client) handleAppStateCommand(args []string) error {
    //
}

func (c *Client) handleRequestAppStateKeyCommand(args []string) error {
    if len(args) < 1 {
			log.Errorf("Usage: request-appstate-key <ids...>")
			return
		}
		var keyIDs = make([][]byte, len(args))
		for i, id := range args {
			decoded, err := hex.DecodeString(id)
			if err != nil {
				log.Errorf("Failed to decode %s as hex: %v", id, err)
				return
			}
			keyIDs[i] = decoded
		}
		cli.DangerousInternals().RequestAppStateKeys(context.Background(), keyIDs)
}

func (c *Client) handleUnavailableRequestCommand(args []string) error {
    if len(args) < 3 {
        log.Errorf("Usage: unavailable-request <chat JID> <sender JID> <message ID>")
        return
    }
    chat, ok := parseJID(args[0])
    if !ok {
        return
    }
    sender, ok := parseJID(args[1])
    if !ok {
        return
    }
    resp, err := cli.SendMessage(
        context.Background(),
        cli.Store.ID.ToNonAD(),
        cli.BuildUnavailableMessageRequest(chat, sender, args[2]),
        whatsmeow.SendRequestExtra{Peer: true},
    )
    fmt.Println(resp)
    fmt.Println(err)
}

func (c *Client) handleCheckUserCommand(args []string) error {
    if len(args) < 1 {
        log.Errorf("Usage: checkuser <phone numbers...>")
        return
    }
    resp, err := cli.IsOnWhatsApp(args)
    if err != nil {
        log.Errorf("Failed to check if users are on WhatsApp: %s", err.Error())
    } else {
        for _, item := range resp {
            if item.VerifiedName != nil {
                log.Infof("%s: on whatsapp: %t, JID: %s, business name: %s", item.Query, item.IsIn, item.JID, item.VerifiedName.Details.GetVerifiedName())
            } else {
                log.Infof("%s: on whatsapp: %t, JID: %s", item.Query, item.IsIn, item.JID)
            }
        }
    }
}

func (c *Client) handleSubscribePresenceCommand(args []string) error {
    if len(args) < 1 {
        log.Errorf("Usage: subscribepresence <jid>")
        return
    }
    jid, ok := parseJID(args[0])
    if !ok {
        return
    }
    err := cli.SubscribePresence(jid)
    if err != nil {
        fmt.Println(err)
    }
}

func (c *Client) handlePresenceCommand(args []string) error {
    if len(args) == 0 {
        log.Errorf("Usage: presence <available/unavailable>")
        return
    }
    fmt.Println(cli.SendPresence(types.Presence(args[0])))
}

func (c *Client) handleChatPresenceCommand(args []string) error {
    if len(args) == 2 {
        args = append(args, "")
    } else if len(args) < 2 {
        log.Errorf("Usage: chatpresence <jid> <composing/paused> [audio]")
        return
    }
    jid, _ := types.ParseJID(args[0])
    fmt.Println(cli.SendChatPresence(jid, types.ChatPresence(args[1]), types.ChatPresenceMedia(args[2])))
}

func (c *Client) handleGetUserCommand(args []string) error {
    if len(args) < 1 {
        log.Errorf("Usage: getuser <jids...>")
        return
    }
    var jids []types.JID
    for _, arg := range args {
        jid, ok := parseJID(arg)
        if !ok {
            return
        }
        jids = append(jids, jid)
    }
    resp, err := cli.GetUserInfo(jids)
    if err != nil {
        log.Errorf("Failed to get user info: %v", err)
    } else {
        for jid, info := range resp {
            log.Infof("%s: %+v", jid, info)
        }
    }
}

func (c *Client) handleRawCommand(args []string) error {
    var node waBinary.Node
		if err := json.Unmarshal([]byte(strings.Join(args, " ")), &node); err != nil {
			log.Errorf("Failed to parse args as JSON into XML node: %v", err)
		} else if err = cli.DangerousInternals().SendNode(node); err != nil {
			log.Errorf("Error sending node: %v", err)
		} else {
			log.Infof("Node sent")
		}
}

func (c *Client) handleQueryBusinessLinkCommand(args []string) error {
    if len(args) < 1 {
        log.Errorf("Usage: querybusinesslink <link>")
        return
    }
    resp, err := cli.ResolveBusinessMessageLink(args[0])
    if err != nil {
        log.Errorf("Failed to resolve business message link: %v", err)
    } else {
        log.Infof("Business info: %+v", resp)
    }
}

func (c *Client) handleListUsersCommand(args []string) error {
    users, err := cli.Store.Contacts.GetAllContacts()
		if err != nil {
			log.Errorf("Failed to get user list: %v", err)
		} else {
			type User struct {
				Found        bool   `json:"Found"`
				FirstName    string `json:"FirstName"`
				FullName     string `json:"FullName"`
				PushName     string `json:"PushName"`
				BusinessName string `json:"BusinessName"`
			}
			jids := make([]string, 0, len(users))
			for jid := range users {
				jids = append(jids, fmt.Sprintf("%s", jid))
			}
			output := struct {
				Jids  []string `json:"jids"`
				Users map[types.JID]types.ContactInfo `json:"users"`
			}{
				Jids:  jids,
				Users: users,
			}
			jsonContent, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Print(string(jsonContent))
		}
}

func (c *Client) handleListGroupsCommand(args []string) error {
    groups, err := cli.GetJoinedGroups()
		if err != nil {
			log.Errorf("Failed to get group list: %v", err)
		} else {
			jsonContent, err := json.MarshalIndent(groups, "", "  ")
			if err != nil {
				fmt.Println(err)
				return
			}
			result := make(map[string]interface{})
			result["groups"] = json.RawMessage(jsonContent)
			output, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Print(string(output))
		}
}

func (c *Client) handleArchiveCommand(args []string) error {
    if len(args) < 2 {
        log.Errorf("Usage: archive <jid> <action>")
        return
    }
    target, ok := parseJID(args[0])
    if !ok {
        return
    }
    action, err := strconv.ParseBool(args[1])
    if err != nil {
        log.Errorf("invalid second argument: %v", err)
        return
    }
    if *isMode != "both" && *isMode != "send" {
        names := []appstate.WAPatchName{appstate.WAPatchName(args[0])}
        new_args := []string{"all"}
        if new_args[0] == "all" {
            names = []appstate.WAPatchName{appstate.WAPatchRegular, appstate.WAPatchRegularHigh, appstate.WAPatchRegularLow, appstate.WAPatchCriticalUnblockLow, appstate.WAPatchCriticalBlock}
        }
        
        resync := len(new_args) > 1 && new_args[1] == "resync"
        for _, name := range names {
            cli.FetchAppState(name, resync, false)
        }
    }
    err = cli.SendAppState(appstate.BuildArchive(target, action, time.Time{}, nil))
    if err != nil {
        log.Errorf("Error changing chat'\''s archive state: %v", err)
    }
}

func (c *Client) handleMuteCommand(args []string) error {
    if len(args) < 2 {
        log.Errorf("Usage: mute <jid> <action> <hours> (default is 8hrs, if 0 then indefinitely)")
        return
    }
    target, ok := parseJID(args[0])
    if !ok {
        return
    }
    action, err := strconv.ParseBool(args[1])
    if err != nil {
        log.Errorf("invalid second argument: %v", err)
        return
    }
    var hours time.Duration
    if len(args) < 3 {
        hours, _ = time.ParseDuration("8h")
    } else {
        t, _ := strconv.ParseInt(args[2], 10, 64)
        if t == 0 {
            hours, _ = time.ParseDuration("318538h")
        } else if t > 0 && t <= 168 {
            hours, _ = time.ParseDuration(fmt.Sprintf("%dh", t))
        } else {
            hours, _ = time.ParseDuration("8h")
        }
    }
    if *isMode != "both" && *isMode != "send" {
        names := []appstate.WAPatchName{appstate.WAPatchName(args[0])}
        new_args := []string{"all"}
        if new_args[0] == "all" {
            names = []appstate.WAPatchName{appstate.WAPatchRegular, appstate.WAPatchRegularHigh, appstate.WAPatchRegularLow, appstate.WAPatchCriticalUnblockLow, appstate.WAPatchCriticalBlock}
        }
        
        resync := len(new_args) > 1 && new_args[1] == "resync"
        for _, name := range names {
            cli.FetchAppState(name, resync, false)
        }
    }
    err = cli.SendAppState(appstate.BuildMute(target, action, hours))
    if err != nil {
        log.Errorf("Error changing chat'\''s mute state: %v", err)
    } else {
        if action {
                log.Infof("Changed mute state for JID: %s, state: %t, duration: %s", target, action, hours)
            } else {
                log.Infof("Changed mute state for JID: %s, state: %t", target, action)
            }
    }
}

func (c *Client) handlePinCommand(args []string) error {
    if len(args) < 2 {
        log.Errorf("Usage: pin <jid> <action>")
        return
    }
    target, ok := parseJID(args[0])
    if !ok {
        return
    }
    action, err := strconv.ParseBool(args[1])
    if err != nil {
        log.Errorf("invalid second argument: %v", err)
        return
    }
    if *isMode != "both" && *isMode != "send" {
        names := []appstate.WAPatchName{appstate.WAPatchName(args[0])}
        new_args := []string{"all"}
        if new_args[0] == "all" {
            names = []appstate.WAPatchName{appstate.WAPatchRegular, appstate.WAPatchRegularHigh, appstate.WAPatchRegularLow, appstate.WAPatchCriticalUnblockLow, appstate.WAPatchCriticalBlock}
        }
        
        resync := len(new_args) > 1 && new_args[1] == "resync"
        for _, name := range names {
            cli.FetchAppState(name, resync, false)
        }
    }
    err = cli.SendAppState(appstate.BuildPin(target, action))
    if err != nil {
        log.Errorf("Error changing chat'\''s pin state: %v", err)
    }
}

func (c *Client) handleLabelChatCommand(args []string) error {
    if len(args) < 3 {
        log.Errorf("Usage: labelchat <jid> <labelID> <action>")
        return
    }
    jid, ok := parseJID(args[0])
    if !ok {
        return
    }
    labelID := args[1]
    action, err := strconv.ParseBool(args[2])
    if err != nil {
        log.Errorf("invalid third argument: %v", err)
        return
    }

    err = cli.SendAppState(appstate.BuildLabelChat(jid, labelID, action))
    if err != nil {
        log.Errorf("Error changing chat'\''s label state: %v", err)
    }
}

func (c *Client) handleLabelMessageCommand(args []string) error {
    if len(args) < 4 {
        log.Errorf("Usage: labelmessage <jid> <labelID> <messageID> <action>")
        return
    }
    jid, ok := parseJID(args[0])
    if !ok {
        return
    }
    labelID := args[1]
    messageID := args[2]
    action, err := strconv.ParseBool(args[3])
    if err != nil {
        log.Errorf("invalid fourth argument: %v", err)
        return
    }

    err = cli.SendAppState(appstate.BuildLabelMessage(jid, labelID, messageID, action))
    if err != nil {
        log.Errorf("Error changing message'\''s label state: %v", err)
    }
}

func (c *Client) handleEditLabelCommand(args []string) error {
    if len(args) < 4 {
        log.Errorf("Usage: editlabel <labelID> <name> <color> <action>")
        return
    }
    labelID := args[0]
    name := args[1]
    color, err := strconv.Atoi(args[2])
    if err != nil {
        log.Errorf("invalid third argument: %v", err)
        return
    }
    action, err := strconv.ParseBool(args[3])
    if err != nil {
        log.Errorf("invalid fourth argument: %v", err)
        return
    }

    err = cli.SendAppState(appstate.BuildLabelEdit(labelID, name, int32(color), action))
    if err != nil {
        log.Errorf("Error editing label: %v", err)
    }
}
