package whatsapp

import (
	"context"
	"strings"

	"go.mau.fi/whatsmeow/types"
	"wahelper/utils"
)

func (c *Client) handleGetGroupCommand(args []string) error {
	if len(args) < 1 {
		c.Logger.Errorf("Usage: getgroup <jid>")
		return nil
	}
	group, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	} else if group.Server != types.GroupServer {
		c.Logger.Errorf("Input must be a group JID (@%s)", types.GroupServer)
		return nil
	}
	resp, err := c.WAClient.GetGroupInfo(group)
	if err != nil {
		c.Logger.Errorf("Failed to get group info: %v", err)
	} else {
		c.Logger.Infof("Group info: %+v", resp)
	}
	return err
}

func (c *Client) handleSubGroupsCommand(args []string) error {
	if len(args) < 1 {
		c.Logger.Errorf("Usage: subgroups <jid>")
		return nil
	}
	group, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	} else if group.Server != types.GroupServer {
		c.Logger.Errorf("Input must be a group JID (@%s)", types.GroupServer)
		return nil
	}
	resp, err := c.WAClient.GetSubGroups(context.Background(), group)
	if err != nil {
		c.Logger.Errorf("Failed to get subgroups: %v", err)
	} else {
		c.Logger.Infof("Subgroups: %+v", resp)
	}
	return err
}

func (c *Client) handleCommunityParticipantsCommand(args []string) error {
	if len(args) < 1 {
		c.Logger.Errorf("Usage: communityparticipants <jid>")
		return nil
	}
	group, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	} else if group.Server != types.GroupServer {
		c.Logger.Errorf("Input must be a group JID (@%s)", types.GroupServer)
		return nil
	}
	resp, err := c.WAClient.GetCommunityParticipants(context.Background(), group)
	if err != nil {
		c.Logger.Errorf("Failed to get community participants: %v", err)
	} else {
		c.Logger.Infof("Community participants: %+v", resp)
	}
	return err
}

func (c *Client) handleGetInviteLinkCommand(args []string) error {
	if len(args) < 1 {
		c.Logger.Errorf("Usage: getinvitelink <jid>")
		return nil
	}
	group, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	} else if group.Server != types.GroupServer {
		c.Logger.Errorf("Input must be a group JID (@%s)", types.GroupServer)
		return nil
	}
	resp, err := c.WAClient.GetGroupInviteLink(group)
	if err != nil {
		c.Logger.Errorf("Failed to get invite link: %v", err)
	} else {
		c.Logger.Infof("Invite link: %s", resp)
	}
	return err
}

func (c *Client) handleQueryInviteLinkCommand(args []string) error {
	if len(args) < 1 {
		c.Logger.Errorf("Usage: queryinvitelink <link>")
		return nil
	}
	resp, err := c.WAClient.QueryGroupInviteLink(args[0])
	if err != nil {
		c.Logger.Errorf("Failed to query invite link: %v", err)
	} else {
		c.Logger.Infof("Invite link info: %+v", resp)
	}
	return err
}

func (c *Client) handleJoinInviteLinkCommand(args []string) error {
	if len(args) < 1 {
		c.Logger.Errorf("Usage: joininvitelink <link>")
		return nil
	}
	resp, err := c.WAClient.JoinGroupWithLink(args[0])
	if err != nil {
		c.Logger.Errorf("Failed to join invite link: %v", err)
	} else {
		c.Logger.Infof("Join invite link response: %+v", resp)
	}
	return err
}

func (c *Client) handleUpdateParticipantCommand(args []string) error {
	if len(args) < 3 {
		c.Logger.Errorf("Usage: updateparticipant <group_jid> <participant_jid> <action>")
		return nil
	}
	group, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	} else if group.Server != types.GroupServer {
		c.Logger.Errorf("Input must be a group JID (@%s)", types.GroupServer)
		return nil
	}
	participant, ok := utils.ParseJID(args[1])
	if !ok {
		return nil
	}
	action := strings.ToLower(args[2])
	var err error
	var resp interface{}

	switch action {
	case "add":
		resp, err = c.WAClient.AddGroupParticipant(group, participant)
	case "remove":
		resp, err = c.WAClient.RemoveGroupParticipant(group, participant)
	case "promote":
		resp, err = c.WAClient.PromoteGroupParticipant(group, participant)
	case "demote":
		resp, err = c.WAClient.DemoteGroupParticipant(group, participant)
	default:
		c.Logger.Errorf("Invalid action: %s. Valid actions are add, remove, promote, demote", action)
		return nil
	}

	if err != nil {
		c.Logger.Errorf("Failed to update participant: %v", err)
	} else {
		c.Logger.Infof("Update participant response: %+v", resp)
	}
	return err
}

func (c *Client) handleGetRequestParticipantCommand(args []string) error {
	if len(args) < 1 {
		c.Logger.Errorf("Usage: getrequestparticipant <jid>")
		return nil
	}
	group, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	} else if group.Server != types.GroupServer {
		c.Logger.Errorf("Input must be a group JID (@%s)", types.GroupServer)
		return nil
	}
	resp, err := c.WAClient.GetGroupJoinRequests(context.Background(), group)
	if err != nil {
		c.Logger.Errorf("Failed to get request participant: %v", err)
	} else {
		c.Logger.Infof("Request participant: %+v", resp)
	}
	return err
}
