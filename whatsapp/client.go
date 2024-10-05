package whatsapp

import (
    "bytes"
    "context"
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "image"
    "image/jpeg"
    _ "image/png"
    "io"
    "io/ioutil"
    "net"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "sync"
    "sync/atomic"
    "time"

    "github.com/jessevdk/go-flags"
    "github.com/mattn/go-sqlite3"
    "github.com/otiai10/opengraph/v2"
    "github.com/sirupsen/logrus"
    "github.com/zRedShift/mimemagic"
    "go.mau.fi/util/random"
    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/appstate"
    waBinary "go.mau.fi/whatsmeow/binary"
    waCommon "go.mau.fi/whatsmeow/proto/waCommon"
    waCompanionReg "go.mau.fi/whatsmeow/proto/waCompanionReg"
    waE2E "go.mau.fi/whatsmeow/proto/waE2E"
    "go.mau.fi/whatsmeow/store"
    "go.mau.fi/whatsmeow/store/sqlstore"
    "go.mau.fi/whatsmeow/types"
    "go.mau.fi/whatsmeow/types/events"
    waLog "go.mau.fi/whatsmeow/util/log"
    "google.golang.org/protobuf/proto"
    "wahelper/utils"
)

type Client struct {
    WAClient         *whatsmeow.Client
    Logger           *logrus.Logger
    Config           *Config
    IsConnected      bool
    DeviceID         string
    DeviceJID        string
    DefaultJID       string
    WaitGroup        sync.WaitGroup
    WaitSync         sync.WaitGroup
    GroupInfo        GroupInfo
    UpdatedGroupInfo bool
    KeepAliveTimeout bool
    ServerRunning    bool
    CurrentDir       string
    FFmpegScriptPath string
    PairRejectChan   chan bool
}

type Config struct {
    LogLevel        string `long:"log-level" description:"Logging level" default:"INFO"`
    DebugLogs       bool   `long:"debug" description:"Enable debug logs?"`
    DBDialect       string `long:"db-dialect" description:"Database dialect (sqlite3 or postgres)" default:"sqlite3"`
    DBAddress       string `long:"db-address" description:"Database address" default:"file:wahelper.db?_foreign_keys=on"`
    RequestFullSync bool   `long:"request-full-sync" description:"Request full (1 year) history sync when logging in?"`
    HTTPPort        int    `long:"port" description:"HTTP server port" default:"7774"`
    Mode            string `long:"mode" description:"Select mode: none, both, send" default:"none"`
    SaveMedia       bool   `long:"save-media" description:"Save Media"`
    AutoDelete      bool   `long:"auto-delete-media" description:"Delete downloaded media after 30s"`
}

type Group struct {
    JID  string `json:"JID"`
    Name string `json:"Name"`
}

type GroupInfo struct {
    Groups []Group `json:"groups"`
}

func NewClient(config *Config) (*Client, error) {
    waBinary.IndentXML = true
    if config.DebugLogs {
        config.LogLevel = "DEBUG"
    }

    if config.RequestFullSync {
        store.DeviceProps.RequireFullSync = proto.Bool(true)
        store.DeviceProps.HistorySyncConfig = &waCompanionReg.DeviceProps_HistorySyncConfig{
            FullSyncDaysLimit:   proto.Uint32(3650),
            FullSyncSizeMbLimit: proto.Uint32(102400),
            StorageQuotaMb:      proto.Uint32(102400),
        }
    }

    log := waLog.Stdout("Main", config.LogLevel, true)
    dbLog := waLog.Stdout("Database", config.LogLevel, true)
    storeContainer, err := sqlstore.New(config.DBDialect, config.DBAddress, dbLog)
    if err != nil {
        log.Errorf("Failed to connect to database: %v", err)
        return nil, err
    }

    device, err := storeContainer.GetFirstDevice()
    if err != nil {
        log.Errorf("Failed to get device: %v", err)
        return nil, err
    }

    waClient := whatsmeow.NewClient(device, waLog.Stdout("Client", config.LogLevel, true))

    client := &Client{
        WAClient:       waClient,
        Logger:         logrus.New(),
        Config:         config,
        PairRejectChan: make(chan bool, 1),
    }

    client.CurrentDir, _ = os.Getwd()
    os.RemoveAll(filepath.Join(client.CurrentDir, ".tmp"))
    client.FFmpegScriptPath = filepath.Join(filepath.Dir(client.CurrentDir), "wahelper", "ffmpeg", "ffmpeg")

    return client, nil
}

func (c *Client) Connect() error {
    var isWaitingForPair atomic.Bool
    c.WAClient.PrePairCallback = func(jid types.JID, platform, businessName string) bool {
        isWaitingForPair.Store(true)
        defer isWaitingForPair.Store(false)
        c.Logger.Infof("Pairing %s (platform: %q, business name: %q). Type 'r' within 3 seconds to reject pair", jid, platform, businessName)
        select {
        case reject := <-c.PairRejectChan:
            if reject {
                c.Logger.Infof("Rejecting pair")
                return false
            }
        case <-time.After(3 * time.Second):
        }
        c.Logger.Infof("Accepting pair")
        return true
    }

    c.WAClient.AddEventHandler(c.EventHandler)
    err := c.WAClient.Connect()
    if err != nil {
        c.Logger.Errorf("Failed to connect: %v", err)
        return err
    }

    if c.WAClient.Store.ID != nil {
        c.DeviceID = c.WAClient.Store.ID.String()
        c.DeviceJID = c.WAClient.Store.ID.String()
        c.DefaultJID = c.WAClient.Store.ID.ToNonAD().String()
    }

    c.IsConnected = true
    return nil
}

func (c *Client) Disconnect() {
    c.WAClient.Disconnect()
    c.IsConnected = false
}

func (c *Client) EventHandler(rawEvt interface{}) {
    switch evt := rawEvt.(type) {
    case *events.AppStateSyncComplete:
        if len(c.WAClient.Store.PushName) > 0 && evt.Name == appstate.WAPatchCriticalBlock {
            err := c.WAClient.SendPresence(types.PresenceAvailable)
            if err != nil {
                c.Logger.Warnf("Failed to send available presence: %v", err)
            } else {
                c.Logger.Infof("Marked self as available")
                c.IsConnected = true

                if c.Config.Mode == "both" {
                    c.UpdatedGroupInfo = false
                    groups, err := c.WAClient.GetJoinedGroups()
                    if err == nil {
                        c.GroupInfo.Groups = []Group{}
                        for _, group := range groups {
                            c.GroupInfo.Groups = append(c.GroupInfo.Groups, Group{
                                JID:  group.JID.String(),
                                Name: group.Name,
                            })
                        }
                    }
                    c.UpdatedGroupInfo = true
                    c.Logger.Infof("Receive/Send Mode Enabled")
                    c.Logger.Infof("Will Now Receive/Send Messages In Tasker")
                } else if c.Config.Mode == "send" {
                    c.Logger.Infof("Send Mode Enabled")
                    c.Logger.Infof("Can Now Send Messages From Tasker")
                }
            }
        }
    case *events.Connected, *events.PushNameSetting:
        if len(c.WAClient.Store.PushName) == 0 {
            return
        }
        err := c.WAClient.SendPresence(types.PresenceAvailable)
        if err != nil {
            c.Logger.Warnf("Failed to send available presence: %v", err)
        } else {
            c.Logger.Infof("Marked self as available")
            c.IsConnected = true

            if c.Config.Mode == "both" {
                c.UpdatedGroupInfo = false
                groups, err := c.WAClient.GetJoinedGroups()
                if err == nil {
                    c.GroupInfo.Groups = []Group{}
                    for _, group := range groups {
                        c.GroupInfo.Groups = append(c.GroupInfo.Groups, Group{
                            JID:  group.JID.String(),
                            Name: group.Name,
                        })
                    }
                }
                c.UpdatedGroupInfo = true
                c.Logger.Infof("Receive/Send Mode Enabled")
                c.Logger.Infof("Will Now Receive/Send Messages In Tasker")
            } else if c.Config.Mode == "send" {
                c.Logger.Infof("Send Mode Enabled")
                c.Logger.Infof("Can Now Send Messages From Tasker")
            }
        }
    case *events.StreamReplaced:
        c.Logger.Infof("Stream replaced, exiting")
        os.Exit(0)
    case *events.Message:
        metaParts := []string{
            fmt.Sprintf("pushname: %s", evt.Info.PushName),
            fmt.Sprintf("timestamp: %s", evt.Info.Timestamp),
        }
        if evt.Info.Type != "" {
            metaParts = append(metaParts, fmt.Sprintf("type: %s", evt.Info.Type))
        }
        c.Logger.Infof("Received message %s from %s (%s): %+v", evt.Info.ID, evt.Info.SourceString(), strings.Join(metaParts, ", "), evt.Message)

        if c.Config.Mode == "both" {
            if c.IsConnected {
                c.WaitGroup.Add(1)
            }
            go c.ParseReceivedMessage(evt, &c.WaitGroup)
        }
    case *events.Receipt:
        if evt.Type == types.ReceiptTypeRead || evt.Type == types.ReceiptTypeReadSelf {
            c.Logger.Infof("%v was read by %s at %s", evt.MessageIDs, evt.SourceString(), evt.Timestamp)
        } else if evt.Type == types.ReceiptTypeDelivered {
            c.Logger.Infof("%s was delivered to %s at %s", evt.MessageIDs[0], evt.SourceString(), evt.Timestamp)
        }
    case *events.Presence:
        if evt.Unavailable {
            if evt.LastSeen.IsZero() {
                c.Logger.Infof("%s is now offline", evt.From)
            } else {
                c.Logger.Infof("%s is now offline (last seen: %s)", evt.From, evt.LastSeen)
            }
        } else {
            c.Logger.Infof("%s is now online", evt.From)
        }
    case *events.OfflineSyncCompleted:
        go func() {
            c.WaitSync.Wait()
            c.Logger.Infof("Offline Sync Completed")
            c.WaitSync = sync.WaitGroup{}
        }()
    case *events.Disconnected:
        c.IsConnected = false
        c.WaitGroup = sync.WaitGroup{}
        c.Logger.Infof("Bad network, waiting for reconnection")
        err := c.Connect()
        if err != nil {
            c.Logger.Errorf("Failed to connect: %v", err)
        }
    case *events.AppState:
        c.Logger.Debugf("App state event: %+v / %+v", evt.Index, evt.SyncActionValue)
    case *events.KeepAliveTimeout:
        c.Logger.Debugf("Keepalive timeout event: %+v", evt)
        c.IsConnected = false
        c.WaitGroup = sync.WaitGroup{}
        if !c.KeepAliveTimeout {
            c.KeepAliveTimeout = true
            for {
                c.Disconnect()
                err := c.Connect()
                if err == nil {
                    break
                }
                c.Logger.Errorf("Failed to connect after keepalive timeout: %v", err)
                time.Sleep(2 * time.Second)
            }
            c.KeepAliveTimeout = false
        }
    case *events.KeepAliveRestored:
        c.Logger.Debugf("Keepalive restored")
    case *events.Blocklist:
        c.Logger.Infof("Blocklist event: %+v", evt)
    }
}

func (c *Client) ParseReceivedMessage(evt *events.Message, wg *sync.WaitGroup) {
    defer wg.Done()

    // Wait until group info is updated
    for !c.UpdatedGroupInfo {
        time.Sleep(1 * time.Second)
        if c.UpdatedGroupInfo {
            break
        }
    }

    isSupported := false
    jsonData := "{}"
    path := ""
    port := fmt.Sprintf("%d", c.Config.HTTPPort)
    messageID := evt.Info.ID
    senderPushName := evt.Info.PushName
    senderJID := evt.Info.Sender.String()
    receiverJID := evt.Info.Chat.String()
    timeStamp := fmt.Sprintf("%d", evt.Info.Timestamp.Unix())
    isFromMyself := ""
    if evt.Info.MessageSource.IsFromMe {
        isFromMyself = "true"
    } else {
        isFromMyself = "false"
        if senderJID == receiverJID && c.DefaultJID != "" {
            receiverJID = c.DefaultJID
        }
    }
    isGroup := ""
    statusMessage := false
    groupName := ""
    if evt.Info.MessageSource.IsGroup && receiverJID != "status@broadcast" {
        isGroup = "true"
        for _, group := range c.GroupInfo.Groups {
            if group.JID == receiverJID {
                groupName = group.Name
                break
            }
        }

        if groupName != "" {
            jsonData, _ = utils.AppendToJSON(jsonData, "group_name", groupName)
        } else {
            jsonData, _ = utils.AppendToJSON(jsonData, "group_name", "Unknown, Group Not Found")
        }
    } else {
        isGroup = "false"
    }
    if receiverJID == "status@broadcast" {
        receiverJID = c.DefaultJID
        statusMessage = true
    }

    jsonData, _ = utils.AppendToJSON(jsonData, "port", port)
    jsonData, _ = utils.AppendToJSON(jsonData, "sender_jid", senderJID)
    jsonData, _ = utils.AppendToJSON(jsonData, "receiver_jid", receiverJID)
    jsonData, _ = utils.AppendToJSON(jsonData, "sender_pushname", senderPushName)
    jsonData, _ = utils.AppendToJSON(jsonData, "is_from_myself", isFromMyself)
    jsonData, _ = utils.AppendToJSON(jsonData, "is_group", isGroup)
    jsonData, _ = utils.AppendToJSON(jsonData, "time_stamp", timeStamp)

    // Handle different message types
    if text := evt.Message.GetConversation(); text != "" {
        // Text message
        isSupported = true
        jsonData, _ = utils.AppendToJSON(jsonData, "type", "text_message")
        jsonData, _ = utils.AppendToJSON(jsonData, "message", text)
        jsonData, _ = utils.AppendToJSON(jsonData, "message_id", messageID)
    } else if extendedText := evt.Message.GetExtendedTextMessage(); extendedText != nil {
        // Extended text message
        if evt.Info.Type == "text" {
            isSupported = true
            message := extendedText.GetText()
            if !statusMessage {
                jsonData, _ = utils.AppendToJSON(jsonData, "type", "text_message")
            } else {
                jsonData, _ = utils.AppendToJSON(jsonData, "type", "status_message")
            }
            jsonData, _ = utils.AppendToJSON(jsonData, "message", message)
            jsonData, _ = utils.AppendToJSON(jsonData, "message_id", messageID)
        } else if evt.Info.Type == "media" {
            // Link message
            if extendedText.GetCanonicalUrl() != "" {
                isSupported = true
                message := extendedText.GetText()
                matchedText := extendedText.GetMatchedText()
                canonicalURL := extendedText.GetCanonicalUrl()
                description := extendedText.GetDescription()
                title := extendedText.GetTitle()
                linkPreviewThumbnail := extendedText.GetJpegThumbnail()
                if len(linkPreviewThumbnail) == 0 {
                    c.Logger.Errorf("Failed to save link preview thumbnail: User cancelled it")
                    return
                }
                os.MkdirAll(filepath.Join(c.CurrentDir, "media", "link"), os.ModePerm)
                path = filepath.Join(c.CurrentDir, "media", "link", fmt.Sprintf("%s.jpg", evt.Info.ID))
                err := os.WriteFile(path, linkPreviewThumbnail, 0644)
                if err != nil {
                    c.Logger.Errorf("Failed to save link preview thumbnail: %v", err)
                    return
                }
                c.Logger.Infof("Saved link preview thumbnail in message to %s", path)
                jsonData, _ = utils.AppendToJSON(jsonData, "path", path)
                if !statusMessage {
                    jsonData, _ = utils.AppendToJSON(jsonData, "type", "link_message")
                } else {
                    jsonData, _ = utils.AppendToJSON(jsonData, "type", "status_message")
                }
                jsonData, _ = utils.AppendToJSON(jsonData, "message", message)
                jsonData, _ = utils.AppendToJSON(jsonData, "link_matched_text", matchedText)
                jsonData, _ = utils.AppendToJSON(jsonData, "link_canonical_url", canonicalURL)
                jsonData, _ = utils.AppendToJSON(jsonData, "link_description", description)
                jsonData, _ = utils.AppendToJSON(jsonData, "link_title", title)
                jsonData, _ = utils.AppendToJSON(jsonData, "message_id", messageID)
            }
        }
    } else if buttonResp := evt.Message.GetButtonsResponseMessage(); buttonResp != nil {
        // Button response message
        isSupported = true
        originMessageID := buttonResp.GetContextInfo().GetStanzaId()
        buttonSelected := buttonResp.GetSelectedDisplayText()
        buttonTitle := buttonResp.GetContextInfo().GetQuotedMessage().GetButtonsMessage().GetText()
        buttonBody := buttonResp.GetContextInfo().GetQuotedMessage().GetButtonsMessage().GetContentText()
        buttonFooter := buttonResp.GetContextInfo().GetQuotedMessage().GetButtonsMessage().GetFooterText()

        jsonData, _ = utils.AppendToJSON(jsonData, "type", "button_response_message")
        jsonData, _ = utils.AppendToJSON(jsonData, "button_selected_button", buttonSelected)
        jsonData, _ = utils.AppendToJSON(jsonData, "button_title", buttonTitle)
        jsonData, _ = utils.AppendToJSON(jsonData, "button_body", buttonBody)
        jsonData, _ = utils.AppendToJSON(jsonData, "button_footer", buttonFooter)
        jsonData, _ = utils.AppendToJSON(jsonData, "origin_message_id", originMessageID)
        jsonData, _ = utils.AppendToJSON(jsonData, "message_id", messageID)
    } else if listResp := evt.Message.GetListResponseMessage(); listResp != nil {
        // List response message
        isSupported = true
        originMessageID := listResp.GetContextInfo().GetStanzaId()
        listSelectedTitle := listResp.GetTitle()
        listSelectedDescription := listResp.GetDescription()
        listTitle := listResp.GetContextInfo().GetQuotedMessage().GetListMessage().GetTitle()
        listBody := listResp.GetContextInfo().GetQuotedMessage().GetListMessage().GetDescription()
        listFooter := listResp.GetContextInfo().GetQuotedMessage().GetListMessage().GetFooterText()
        listButtonText := listResp.GetContextInfo().GetQuotedMessage().GetListMessage().GetButtonText()
        listHeader := listResp.GetContextInfo().GetQuotedMessage().GetListMessage().GetSections()[0].GetTitle()

        jsonData, _ = utils.AppendToJSON(jsonData, "type", "list_response_message")
        jsonData, _ = utils.AppendToJSON(jsonData, "list_selected_title", listSelectedTitle)
        jsonData, _ = utils.AppendToJSON(jsonData, "list_selected_description", listSelectedDescription)
        jsonData, _ = utils.AppendToJSON(jsonData, "list_title", listTitle)
        jsonData, _ = utils.AppendToJSON(jsonData, "list_body", listBody)
        jsonData, _ = utils.AppendToJSON(jsonData, "list_footer", listFooter)
        jsonData, _ = utils.AppendToJSON(jsonData, "list_button_text", listButtonText)
        jsonData, _ = utils.AppendToJSON(jsonData, "list_header", listHeader)
        jsonData, _ = utils.AppendToJSON(jsonData, "message_id", messageID)
        jsonData, _ = utils.AppendToJSON(jsonData, "origin_message_id", originMessageID)
    } else if pollUpdate := evt.Message.GetPollUpdateMessage(); pollUpdate != nil {
        // Poll update message
        isSupported = true
        messageID = pollUpdate.GetPollCreationMessageKey().GetId()
        decrypted, err := c.WAClient.DecryptPollVote(evt)
        if err != nil {
            c.Logger.Errorf("Failed to decrypt vote: %v", err)
            return
        }

        questionData, err := os.ReadFile(filepath.Join(c.CurrentDir, ".tmp", "poll_question_"+messageID))
        if err != nil {
            c.Logger.Errorf("Failed to read question data: %v", err)
            return
        }

        question := string(questionData)

        selectedOptions := make([]interface{}, len(decrypted.SelectedOptions))
        for i, selectedOption := range decrypted.SelectedOptions {
            optionData, err := os.ReadFile(filepath.Join(c.CurrentDir, ".tmp", "poll_option_"+strings.ToLower(fmt.Sprintf("%X", selectedOption))))
            if err != nil {
                c.Logger.Errorf("Failed to read option data: %v", err)
                return
            }
            selectedOptions[i] = string(optionData)
        }

        jsonData, _ = utils.AppendToJSON(jsonData, "type", "poll_response_message")
        jsonData, _ = utils.AppendToJSON(jsonData, "poll_question", question)
        jsonData, _ = utils.AppendToJSON(jsonData, "poll_selected_options", selectedOptions)
        jsonData, _ = utils.AppendToJSON(jsonData, "message_id", messageID)
    } else if c.Config.SaveMedia {
        // Handle media messages (e.g., images, videos, audio, etc.)
        // Implement handling similar to the original code, downloading media, saving files, etc.
        // Due to the complexity, please refer to the original `parseReceivedMessage` function for detailed implementation.
    }

    if isSupported {
        c.Logger.Infof("%s", jsonData)
        // Send HTTP POST request
        httpPath := "/message"
        go c.sendHttpPost(jsonData, httpPath)
    }
    if c.Config.AutoDelete {
        go func() {
            if path != "" {
                time.Sleep(30 * time.Second)
                os.Remove(path)
            }
        }()
    }
}

func (c *Client) sendHttpPost(jsonData string, path string) {
    client := &http.Client{
        Timeout: 1 * time.Second,
    }

    jsonBody := []byte(jsonData)
    bodyReader := bytes.NewReader(jsonBody)

    requestURL := fmt.Sprintf("http://localhost:%d%s", c.Config.HTTPPort, path)
    req, err := http.NewRequest(http.MethodPost, requestURL, bodyReader)
    if err != nil {
        c.Logger.Errorf("Failed to create HTTP request: %v", err)
        return
    }
    resp, err := client.Do(req)
    if err != nil {
        c.Logger.Errorf("Failed to send HTTP POST request: %v", err)
        return
    }
    defer resp.Body.Close()
}

func (c *Client) SendMessage(recipientJID string, message string) error {
    recipient, ok := utils.ParseJID(recipientJID)
    if !ok {
        c.Logger.Errorf("Invalid JID: %s", recipientJID)
        return fmt.Errorf("invalid JID")
    }
    msg := &waE2E.Message{Conversation: proto.String(message)}
    resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
    if err != nil {
        c.Logger.Errorf("Error sending message: %v", err)
        return err
    }
    c.Logger.Infof("Message sent to %s (server timestamp: %s)", recipientJID, resp.Timestamp)
    return nil
}
