package handlers

import (
    "strings"
    "wahelper/utils"
    "wahelper/whatsapp"
)

func HandleCommand(waClient *whatsapp.Client, cmd string, args []string) {
    switch cmd {
    case "send":
        if len(args) < 2 {
            waClient.Logger.Error("Usage: send <jid> <message>")
            return
        }
        recipientJID := args[0]
        message := strings.Join(args[1:], " ")
        err := waClient.SendMessage(recipientJID, message)
        if err != nil {
            waClient.Logger.Errorf("Failed to send message: %v", err)
        }
    case "pair-phone":
        if len(args) < 1 {
            waClient.Logger.Error("Usage: pair-phone <number>")
            return
        }
        if !waClient.WAClient.IsConnected() {
            waClient.Logger.Error("Not connected to WhatsApp")
            return
        }
        if waClient.WAClient.IsLoggedIn() {
            waClient.Logger.Info("Already paired")
            return
        }
        linkingCode, err := waClient.WAClient.PairPhone(args[0], true, whatsmeow.PairClientUnknown, "Firefox (Android)")
        if err != nil {
            waClient.Logger.Errorf("Error pairing phone: %v", err)
            return
        }
        waClient.Logger.Infof(`Linking code: "%s"`, linkingCode)
    case "logout":
        err := waClient.WAClient.Logout()
        if err != nil {
            waClient.Logger.Errorf("Error logging out: %v", err)
        } else {
            waClient.Logger.Infof("Successfully logged out")
        }
    case "appstate":
        if len(args) < 1 {
            waClient.Logger.Error("Usage: appstate <types...>")
            return
        }
        names := []appstate.WAPatchName{appstate.WAPatchName(args[0])}
        if args[0] == "all" {
            names = []appstate.WAPatchName{appstate.WAPatchRegular, appstate.WAPatchRegularHigh, appstate.WAPatchRegularLow, appstate.WAPatchCriticalUnblockLow, appstate.WAPatchCriticalBlock}
        }
        resync := len(args) > 1 && args[1] == "resync"
        for _, name := range names {
            err := waClient.WAClient.FetchAppState(name, resync, false)
            if err != nil {
                waClient.Logger.Errorf("Failed to sync app state: %v", err)
            }
        }
    // TODO: Add other command cases
     default:
         waClient.Logger.Warnf("Unknown command: %s", cmd)
     }
 }
