package whatsapp

func (c *Client) handleMediaConnCommand(args []string) error {
    conn, err := c.cli.DangerousInternals().RefreshMediaConn(false)
    if err != nil {
        c.logger.Errorf("Failed to get media connection: %v", err)
    } else {
        c.logger.Infof("Media connection: %+v", conn)
    }
    return nil
}

func (c *Client) handleGetAvatarCommand(args []string) error {
    if len(args) < 1 {
        c.Logger.Errorf("Usage: getavatar <jid> [existing ID] [--preview] [--community]")
        return
    }
    jid, ok := parseJID(args[0])
    if !ok {
        return
    }
    existingID := ""
    if len(args) > 2 {
        existingID = args[2]
    }
    var preview, isCommunity bool
    for _, arg := range args {
        if arg == "--preview" {
            preview = true
        } else if arg == "--community" {
            isCommunity = true
        }
    }
    pic, err := c.cli.GetProfilePictureInfo(jid, &whatsmeow.GetProfilePictureParams{
        Preview:     preview,
        IsCommunity: isCommunity,
        ExistingID:  existingID,
    })
    if err != nil {
        c.Logger.Errorf("Failed to get avatar: %v", err)
    } else if pic != nil {
        c.Logger.Infof("Got avatar ID %s: %s", pic.ID, pic.URL)
    } else {
        c.Logger.Infof("No avatar found")
    }
}
