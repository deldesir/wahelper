package whatsapp

import (
    "context"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "strconv"
    "strings"
    "time"

    "go.mau.fi/whatsmeow/appstate"
    "go.mau.fi/whatsmeow/binary/waBinary"
    "go.mau.fi/whatsmeow/types"
    "go.mau.fi/whatsmeow/types/events"
    "wahelper/utils"
)

func (c *Client) handleReconnectCommand(args []string) error {
    c.IsConnected = false
    c.WAClient.Disconnect()
    err := c.WAClient.Connect()
    if err != nil {
        c.Logger.Errorf("Failed to reconnect: %v", err)
    }
    return err
}

func (c *Client) handleAppStateCommand(args []string) error {
    if len(args) == 0 {
        c.Logger.Errorf("Usage: appstate [resync] <names...>")
        return nil
    }
    resync := false
    names := []appstate.WAPatchName{}
    for _, arg := range args {
        if arg == "resync" {
            resync = true
        } else {
            names = append(names, appstate.WAPatchName(arg))
        }
    }
    if len(names) == 0 {
        c.Logger.Errorf("No patch names provided.")
        return nil
    }
    for _, name := range names {
        c.WAClient.FetchAppState(name, resync, false)
    }
    return nil
}

func (c *Client) handleRequestAppStateKeyCommand(args []string) error {
    if len(args) < 1 {
        c.Logger.Errorf("Usage: request-appstate-key <ids...>")
        return nil
    }
    var keyIDs = make([][]byte, len(args))
    for i, id := range args {
        decoded, err := hex.DecodeString(id)
        if err != nil {
            c.Logger.Errorf("Failed to decode %s as hex: %v", id, err)
            return nil
        }
        keyIDs[i] = decoded
    }
    c.WAClient.DangerousInternals().RequestAppStateKeys(context.Background(), keyIDs)
    return nil
}

func (c *Client) handleUnavailableRequestCommand(args []string) error {
    if len(args) < 3 {
        c.Logger.Errorf("Usage: unavailable-request <chat JID> <sender JID> <message ID>")
        return nil
    }
    chat, ok := utils.ParseJID(args[0])
    if !ok {
        return nil
    }
    sender, ok := utils.ParseJID(args[1])
    if !ok {
        return nil
    }
    msg := c.WAClient.BuildUnavailableMessageRequest(chat, sender, args[2])
    resp, err := c.WAClient.SendMessage(
        context.Background(),
        c.WAClient.Store.ID.ToNonAD(),
        msg,
        types.SendRequestExtra{Peer: true},
    )
    if err != nil {
        c.Logger.Errorf("Error sending unavailable request: %v", err)
    } else {
        c.Logger.Infof("Unavailable request sent: %+v", resp)
    }
    return err
}

func (c *Client) handleCheckUserCommand(args []string) error {
    if len(args) < 1 {
        c.Logger.Errorf("Usage: checkuser <phone numbers...>")
        return nil
    }
    resp, err := c.WAClient.IsOnWhatsApp(args)
    if err != nil {
        c.Logger.Errorf("Failed to check if users are on WhatsApp: %s", err.Error())
    } else {
        for _, item := range resp {
            if item.VerifiedName != nil {
                c.Logger.Infof("%s: on WhatsApp: %t, JID: %s, business name: %s", item.Query, item.IsIn, item.JID, item.VerifiedName.Details.GetVerifiedName())
            } else {
                c.Logger.Infof("%s: on WhatsApp: %t, JID: %s", item.Query, item.IsIn, item.JID)
            }
        }
    }
    return err
}

func (c *Client) handleSubscribePresenceCommand(args []string) error {
    if len(args) < 1 {
        c.Logger.Errorf("Usage: subscribepresence <jid>")
        return nil
    }
    jid, ok := utils.ParseJID(args[0])
    if !ok {
        return nil
    }
    err := c.WAClient.SubscribePresence(jid)
    if err != nil {
        c.Logger.Errorf("Error subscribing to presence: %v", err)
    } else {
        c.Logger.Infof("Subscribed to presence updates for %s", jid)
    }
    return err
}

func (c *Client) handlePresenceCommand(args []string) error {
    if len(args) == 0 {
        c.Logger.Errorf("Usage: presence <available/unavailable>")
        return nil
    }
    err := c.WAClient.SendPresence(types.Presence(args[0]))
    if err != nil {
        c.Logger.Errorf("Error sending presence: %v", err)
    } else {
        c.Logger.Infof("Presence set to %s", args[0])
    }
    return err
}

func (c *Client) handleChatPresenceCommand(args []string) error {
    if len(args) < 2 {
        c.Logger.Errorf("Usage: chatpresence <jid> <composing/paused> [audio]")
        return nil
    }
    jid, ok := utils.ParseJID(args[0])
    if !ok {
        return nil
    }
    presence := types.ChatPresence(args[1])
    media := types.ChatPresenceMedia("")
    if len(args) > 2 {
        media = types.ChatPresenceMedia(args[2])
    }
    err := c.WAClient.SendChatPresence(jid, presence, media)
    if err != nil {
        c.Logger.Errorf("Error sending chat presence: %v", err)
    } else {
        c.Logger.Infof("Chat presence sent to %s", jid)
    }
    return err
}

func (c *Client) handleGetUserCommand(args []string) error {
    if len(args) < 1 {
        c.Logger.Errorf("Usage: getuser <jids...>")
        return nil
    }
    var jids []types.JID
    for _, arg := range args {
        jid, ok := utils.ParseJID(arg)
        if !ok {
            return nil
        }
        jids = append(jids, jid)
    }
    resp, err := c.WAClient.GetUserInfo(jids)
    if err != nil {
        c.Logger.Errorf("Failed to get user info: %v", err)
    } else {
        for jid, info := range resp {
            c.Logger.Infof("%s: %+v", jid, info)
        }
    }
    return err
}

func (c *Client) handleRawCommand(args []string) error {
    var node waBinary.Node
    if err := json.Unmarshal([]byte(strings.Join(args, " ")), &node); err != nil {
        c.Logger.Errorf("Failed to parse args as JSON into XML node: %v", err)
    } else if err = c.WAClient.DangerousInternals().SendNode(node); err != nil {
        c.Logger.Errorf("Error sending node: %v", err)
    } else {
        c.Logger.Infof("Node sent")
    }
    return nil
}

func (c *Client) handleQueryBusinessLinkCommand(args []string) error {
    if len(args) < 1 {
        c.Logger.Errorf("Usage: querybusinesslink <link>")
        return nil
    }
    resp, err := c.WAClient.ResolveBusinessMessageLink(args[0])
    if err != nil {
        c.Logger.Errorf("Failed to resolve business message link: %v", err)
    } else {
        c.Logger.Infof("Business info: %+v", resp)
    }
    return err
}

func (c *Client) handleListUsersCommand(args []string) error {
    users, err := c.WAClient.Store.Contacts.GetAllContacts()
    if err != nil {
        c.Logger.Errorf("Failed to get user list: %v", err)
    } else {
        jids := make([]string, 0, len(users))
        for jid := range users {
            jids = append(jids, jid.String())
        }
        output := struct {
            JIDs  []string                        `json:"jids"`
            Users map[types.JID]types.ContactInfo `json:"users"`
        }{
            JIDs:  jids,
            Users: users,
        }
        jsonContent, err := json.MarshalIndent(output, "", "  ")
        if err != nil {
            c.Logger.Errorf("Error marshaling users to JSON: %v", err)
            return err
        }
        fmt.Print(string(jsonContent))
    }
    return err
}

func (c *Client) handleListGroupsCommand(args []string) error {
    groups, err := c.WAClient.GetJoinedGroups()
    if err != nil {
        c.Logger.Errorf("Failed to get group list: %v", err)
    } else {
        jsonContent, err := json.MarshalIndent(groups, "", "  ")
        if err != nil {
            c.Logger.Errorf("Error marshaling groups to JSON: %v", err)
            return err
        }
        result := map[string]interface{}{
            "groups": json.RawMessage(jsonContent),
        }
        output, err := json.MarshalIndent(result, "", "  ")
        if err != nil {
            c.Logger.Errorf("Error marshaling result to JSON: %v", err)
            return err
        }
        fmt.Print(string(output))
    }
    return err
}

func (c *Client) handleArchiveCommand(args []string) error {
    if len(args) < 2 {
        c.Logger.Errorf("Usage: archive <jid> <true/false>")
        return nil
    }
    target, ok := utils.ParseJID(args[0])
    if !ok {
        return nil
    }
    action, err := strconv.ParseBool(args[1])
    if err != nil {
        c.Logger.Errorf("Invalid second argument: %v", err)
        return nil
    }
    err = c.WAClient.SendAppState(appstate.BuildArchive(target, action, time.Time{}, nil))
    if err != nil {
        c.Logger.Errorf("Error changing chat's archive state: %v", err)
    } else {
        c.Logger.Infof("Archive state changed for %s to %t", target, action)
    }
    return err
}

func (c *Client) handleMuteCommand(args []string) error {
    if len(args) < 2 {
        c.Logger.Errorf("Usage: mute <jid> <true/false> [hours] (default is 8hrs, if 0 then indefinitely)")
        return nil
    }
    target, ok := utils.ParseJID(args[0])
    if !ok {
        return nil
    }
    action, err := strconv.ParseBool(args[1])
    if err != nil {
        c.Logger.Errorf("Invalid second argument: %v", err)
        return nil
    }
    var duration time.Duration
    if len(args) > 2 {
        t, err := strconv.ParseInt(args[2], 10, 64)
        if err != nil {
            c.Logger.Errorf("Invalid duration: %v", err)
            return nil
        }
        if t == 0 {
            duration = 0 // Indefinite mute
        } else {
            duration = time.Duration(t) * time.Hour
        }
    } else {
        duration = 8 * time.Hour
    }
    err = c.WAClient.SendAppState(appstate.BuildMute(target, action, duration))
    if err != nil {
        c.Logger.Errorf("Error changing chat's mute state: %v", err)
    } else {
        c.Logger.Infof("Mute state changed for %s to %t for %s", target, action, duration)
    }
    return err
}

func (c *Client) handlePinCommand(args []string) error {
    if len(args) < 2 {
        c.Logger.Errorf("Usage: pin <jid> <true/false>")
        return nil
    }
    target, ok := utils.ParseJID(args[0])
    if !ok {
        return nil
    }
    action, err := strconv.ParseBool(args[1])
    if err != nil {
        c.Logger.Errorf("Invalid second argument: %v", err)
        return nil
    }
    err = c.WAClient.SendAppState(appstate.BuildPin(target, action))
    if err != nil {
        c.Logger.Errorf("Error changing chat's pin state: %v", err)
    } else {
        c.Logger.Infof("Pin state changed for %s to %t", target, action)
    }
    return err
}

func (c *Client) handleLabelChatCommand(args []string) error {
    if len(args) < 3 {
        c.Logger.Errorf("Usage: labelchat <jid> <labelID> <true/false>")
        return nil
    }
    jid, ok := utils.ParseJID(args[0])
    if !ok {
        return nil
    }
    labelID := args[1]
    action, err := strconv.ParseBool(args[2])
    if err != nil {
        c.Logger.Errorf("Invalid third argument: %v", err)
        return nil
    }
    err = c.WAClient.SendAppState(appstate.BuildLabelChat(jid, labelID, action))
    if err != nil {
        c.Logger.Errorf("Error changing chat's label state: %v", err)
    } else {
        c.Logger.Infof("Label state changed for chat %s, label ID %s, action %t", jid, labelID, action)
    }
    return err
}

func (c *Client) handleLabelMessageCommand(args []string) error {
    if len(args) < 4 {
        c.Logger.Errorf("Usage: labelmessage <jid> <labelID> <messageID> <true/false>")
        return nil
    }
    jid, ok := utils.ParseJID(args[0])
    if !ok {
        return nil
    }
    labelID := args[1]
    messageID := args[2]
    action, err := strconv.ParseBool(args[3])
    if err != nil {
        c.Logger.Errorf("Invalid fourth argument: %v", err)
        return nil
    }
    err = c.WAClient.SendAppState(appstate.BuildLabelMessage(jid, labelID, messageID, action))
    if err != nil {
        c.Logger.Errorf("Error changing message's label state: %v", err)
    } else {
        c.Logger.Infof("Label state changed for message %s in chat %s, label ID %s, action %t", messageID, jid, labelID, action)
    }
    return err
}

func (c *Client) handleEditLabelCommand(args []string) error {
    if len(args) < 4 {
        c.Logger.Errorf("Usage: editlabel <labelID> <name> <color> <true/false>")
        return nil
    }
    labelID := args[0]
    name := args[1]
    color, err := strconv.Atoi(args[2])
    if err != nil {
        c.Logger.Errorf("Invalid third argument: %v", err)
        return nil
    }
    action, err := strconv.ParseBool(args[3])
    if err != nil {
        c.Logger.Errorf("Invalid fourth argument: %v", err)
        return nil
    }
    err = c.WAClient.SendAppState(appstate.BuildLabelEdit(labelID, name, int32(color), action))
    if err != nil {
        c.Logger.Errorf("Error editing label: %v", err)
    } else {
        c.Logger.Infof("Label edited: label ID %s, name %s, color %d, action %t", labelID, name, color, action)
    }
    return err
}
