
package whatsapp

func (c *Client) handleGetGroupCommand(args []string) error {
    if len(args) < 1 {
        c.logger.Errorf("Usage: getgroup <jid>")
        return nil
    }
    group, ok := c.parseJID(args[0])
    if !ok {
        return nil
    } else if group.Server != types.GroupServer {
        c.logger.Errorf("Input must be a group JID (@%s)", types.GroupServer)
        return nil
    }
    resp, err := c.cli.GetGroupInfo(group)
    if err != nil {
        c.logger.Errorf("Failed to get group info: %v", err)
    } else {
        c.logger.Infof("Group info: %+v", resp)
    }
    return nil
}

func (c *Client) handleSubGroupsCommand(args []string) error {
    if len(args) < 1 {
        c.logger.Errorf("Usage: subgroups <jid>")
        return nil
    }
    group, ok := c.parseJID(args[0])
    if !ok {
        return nil
    } else if group.Server != types.GroupServer {
        c.logger.Errorf("Input must be a group JID (@%s)", types.GroupServer)
        return nil
    }
    resp, err := c.cli.GetSubGroups(group)
    if err != nil {
        c.logger.Errorf("Failed to get subgroups: %v", err)
    } else {
        c.logger.Infof("Subgroups: %+v", resp)
    }
    return nil
}

func (c *Client) handleCommunityParticipantsCommand(args []string) error {
    if len(args) < 1 {
        c.logger.Errorf("Usage: communityparticipants <jid>")
        return nil
    }
    group, ok := c.parseJID(args[0])
    if !ok {
        return nil
    } else if group.Server != types.GroupServer {
        c.logger.Errorf("Input must be a group JID (@%s)", types.GroupServer)
        return nil
    }
    resp, err := c.cli.GetCommunityParticipants(group)
    if err != nil {
        c.logger.Errorf("Failed to get community participants: %v", err)
    } else {
        c.logger.Infof("Community participants: %+v", resp)
    }
    return nil
}

func (c *Client) handleGetInviteLinkCommand(args []string) error {
    if len(args) < 1 {
        c.logger.Errorf("Usage: getinvitelink <jid>")
        return nil
    }
    group, ok := c.parseJID(args[0])
    if !ok {
        return nil
    } else if group.Server != types.GroupServer {
        c.logger.Errorf("Input must be a group JID (@%s)", types.GroupServer)
        return nil
    }
    resp, err := c.cli.GetInviteLink(group)
    if err != nil {
        c.logger.Errorf("Failed to get invite link: %v", err)
    } else {
        c.logger.Infof("Invite link: %s", resp)
    }
    return nil
}

func (c *Client) handleQueryInviteLinkCommand(args []string) error {
    if len(args) < 1 {
        c.logger.Errorf("Usage: queryinvitelink <link>")
        return nil
    }
    resp, err := c.cli.QueryInviteLink(args[0])
    if err != nil {
        c.logger.Errorf("Failed to query invite link: %v", err)
    } else {
        c.logger.Infof("Invite link info: %+v", resp)
    }
    return nil
}

func (c *Client) handleJoinInviteLinkCommand(args []string) error {
    if len(args) < 1 {
        c.logger.Errorf("Usage: joininvitelink <link>")
        return nil
    }
    resp, err := c.cli.JoinInviteLink(args[0])
    if err != nil {
        c.logger.Errorf("Failed to join invite link: %v", err)
    } else {
        c.logger.Infof("Join invite link response: %+v", resp)
    }
    return nil
}

func (c *Client) handleUpdateParticipantCommand(args []string) error {
    if len(args) < 2 {
        c.logger.Errorf("Usage: updateparticipant <jid> <role>")
        return nil
    }
    group, ok := c.parseJID(args[0])
    if !ok {
        return nil
    } else if group.Server != types.GroupServer {
        c.logger.Errorf("Input must be a group JID (@%s)", types.GroupServer)
        return nil
    }
    resp, err := c.cli.UpdateParticipant(group, args[1])
    if err != nil {
        c.logger.Errorf("Failed to update participant: %v", err)
    } else {
        c.logger.Infof("Update participant response: %+v", resp)
    }
    return nil
}

func (c *Client) handleGetRequestParticipantCommand(args []string) error {
    if len(args) < 1 {
        c.logger.Errorf("Usage: getrequestparticipant <jid>")
        return nil
    }
    group, ok := c.parseJID(args[0])
    if !ok {
        return nil
    } else if group.Server != types.GroupServer {
        c.logger.Errorf("Input must be a group JID (@%s)", types.GroupServer)
        return nil
    }
    resp, err := c.cli.GetRequestParticipant(group)
    if err != nil {
        c.logger.Errorf("Failed to get request participant: %v", err)
    } else {
        c.logger.Infof("Request participant: %+v", resp)
    }
    return nil
}

