package utils

import (
    "go.mau.fi/whatsmeow/types"
    "strings"
)

// ParseJID parses a string into a WhatsApp JID.
func ParseJID(arg string) (types.JID, bool) {
    if arg[0] == '+' {
        arg = arg[1:]
    }
    if !strings.ContainsRune(arg, '@') {
        return types.NewJID(arg, types.DefaultUserServer), true
    }
    jid, err := types.ParseJID(arg)
    if err != nil {
        return jid, false
    }
    if jid.User == "" {
        return jid, false
    }
    return jid, true
}
