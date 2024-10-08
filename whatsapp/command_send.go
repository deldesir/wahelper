package whatsapp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nfnt/resize"
	"github.com/otiai10/opengraph/v2"
	"github.com/zRedShift/mimemagic"
	"go.mau.fi/util/random"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	waBinary "go.mau.fi/whatsmeow/binary/waBinary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
	"wahelper/utils"
)

func (c *Client) handleSendCommand(args []string) error {
	if len(args) < 2 {
		c.Logger.Errorf("Usage: send <jid> <text>")
		return nil
	}
	recipient, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	}
	msg := &waProto.Message{Conversation: proto.String(strings.Join(args[1:], " "))}
	resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
	if err != nil {
		c.Logger.Errorf("Error sending message: %v", err)
	} else {
		c.Logger.Infof("Message sent (server timestamp: %s)", resp.Timestamp)
	}
	return nil
}

func (c *Client) handleSendListCommand(args []string) error {
	if len(args) < 9 {
		c.Logger.Errorf("Usage: sendlist <jid> <title> <text> <footer> <button text> <sub title> -- <heading 1> <description 1> / ...")
		return nil
	}
	recipient, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	}

	if args[6] != "--" {
		c.Logger.Errorf("Missing -- separator")
		c.Logger.Errorf("Usage: sendlist <jid> <title> <text> <footer> <button text> <sub title> -- <heading 1> <description 1> / ...")
		return nil
	}

	msg := &waProto.Message{
		ListMessage: &waProto.ListMessage{
			Title:       proto.String(args[1]),
			Description: proto.String(args[2]),
			FooterText:  proto.String(args[3]),
			ButtonText:  proto.String(args[4]),
			ListType:    waProto.ListMessage_SINGLE_SELECT.Enum(),
			Sections: []*waProto.ListMessage_Section{
				{
					Title: proto.String(args[5]),
					Rows:  []*waProto.ListMessage_Row{},
				},
			},
		},
	}

	items := args[7:]

	if len(items)%3 != 0 {
		c.Logger.Errorf("Invalid number of items")
		return nil
	}

	for i := 0; i < len(items); i += 3 {
		if items[i+2] != "/" {
			c.Logger.Errorf("Missing '/' separator at position %d", i+2)
			return nil
		}
		newRow := &waProto.ListMessage_Row{
			RowId:       proto.String(fmt.Sprintf("id%d", i/3+1)),
			Title:       proto.String(items[i]),
			Description: proto.String(items[i+1]),
		}
		msg.ListMessage.Sections[0].Rows = append(msg.ListMessage.Sections[0].Rows, newRow)
	}

	resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
	if err != nil {
		c.Logger.Errorf("Error sending list message: %v", err)
	} else {
		c.Logger.Infof("List message sent (server timestamp: %s)", resp.Timestamp)
	}
	return nil
}

func (c *Client) handleSendPollCommand(args []string) error {
	if len(args) < 7 {
		c.Logger.Errorf("Usage: sendpoll <jid> <max answers> <question> -- <option 1> / <option 2> / ...")
		return nil
	}
	recipient, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	}
	maxAnswers, err := strconv.Atoi(args[1])
	if err != nil {
		c.Logger.Errorf("Number of max answers must be an integer")
		return nil
	}
	remainingArgs := strings.Join(args[2:], " ")
	question, optionsStr, found := strings.Cut(remainingArgs, "--")
	if !found {
		c.Logger.Errorf("Missing '--' separator")
		return nil
	}
	question = strings.TrimSpace(question)
	options := strings.Split(optionsStr, "/")
	for i := range options {
		options[i] = strings.TrimSpace(options[i])
	}

	resp, err := c.WAClient.SendMessage(context.Background(), recipient, c.WAClient.BuildPollCreation(question, options, maxAnswers))
	if err != nil {
		c.Logger.Errorf("Error sending poll message: %v", err)
	} else {
		c.Logger.Infof("Poll message sent (server timestamp: %s)", resp.Timestamp)
	}
	return nil
}

func (c *Client) handleSendLinkCommand(args []string) error {
	if len(args) < 2 {
		c.Logger.Errorf("Usage: sendlink <jid> <url/link> [text]")
		return nil
	}
	recipient, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	}

	text := ""
	if len(args) > 2 {
		text = "\n\n" + strings.Join(args[2:], " ")
	}

	ogp, err := opengraph.Fetch(args[1])
	if err != nil || ogp.Title == "" || ogp.Description == "" || len(ogp.Image) == 0 || ogp.Image[0].URL == "" {
		c.Logger.Errorf("Could not fetch Open Graph data: %s", err)
		msg := &waProto.Message{ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text:         proto.String(args[1] + text),
			CanonicalUrl: proto.String(args[1]),
			MatchedText:  proto.String(args[1]),
		}}
		resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
		if err != nil {
			c.Logger.Errorf("Error sending link message: %v", err)
		} else {
			c.Logger.Infof("Link message sent (server timestamp: %s)", resp.Timestamp)
		}
		return nil
	}

	ogp.ToAbs()
	data, err := http.Get(ogp.Image[0].URL)
	if err != nil || data.StatusCode != http.StatusOK {
		c.Logger.Errorf("Could not fetch thumbnail data")
		return nil
	}

	jpegBytes, err := io.ReadAll(data.Body)
	data.Body.Close()
	if err != nil {
		c.Logger.Errorf("Could not read thumbnail data: %s", err)
		return nil
	}

	config, _, err := image.DecodeConfig(bytes.NewReader(jpegBytes))
	if err != nil {
		c.Logger.Errorf("Could not decode image config: %s", err)
		return nil
	}

	thumbnailResp, err := c.WAClient.Upload(context.Background(), jpegBytes, whatsmeow.MediaImage)
	if err != nil {
		c.Logger.Errorf("Failed to upload thumbnail: %v", err)
		return nil
	}

	msg := &waProto.Message{ExtendedTextMessage: &waProto.ExtendedTextMessage{
		Text:                proto.String(args[1] + text),
		Title:               proto.String(ogp.Title),
		CanonicalUrl:        proto.String(args[1]),
		MatchedText:         proto.String(args[1]),
		Description:         proto.String(ogp.Description),
		JpegThumbnail:       jpegBytes,
		ThumbnailDirectPath: proto.String(thumbnailResp.DirectPath),
		ThumbnailSha256:     thumbnailResp.FileSHA256,
		ThumbnailEncSha256:  thumbnailResp.FileEncSHA256,
		ThumbnailWidth:      proto.Uint32(uint32(config.Width)),
		ThumbnailHeight:     proto.Uint32(uint32(config.Height)),
		MediaKey:            thumbnailResp.MediaKey,
	}}
	resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
	if err != nil {
		c.Logger.Errorf("Error sending link message: %v", err)
	} else {
		c.Logger.Infof("Link message sent (server timestamp: %s)", resp.Timestamp)
	}
	return nil
}

func (c *Client) handleSendDocumentCommand(args []string) error {
	if len(args) < 3 {
		c.Logger.Errorf("Usage: senddoc <jid> <document path> <document file name> [caption] [mime-type]")
		return nil
	}
	recipient, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	}
	data, err := os.ReadFile(args[1])
	if err != nil {
		c.Logger.Errorf("Failed to read %s: %v", args[1], err)
		return nil
	}
	uploaded, err := c.WAClient.Upload(context.Background(), data, whatsmeow.MediaDocument)
	if err != nil {
		c.Logger.Errorf("Failed to upload file: %v", err)
		return nil
	}
	caption := ""
	if len(args) > 3 && args[3] != "" {
		caption = args[3]
	}
	mimeType := http.DetectContentType(data)
	if len(args) > 4 {
		mimeType = args[4]
	}
	msg := &waProto.Message{DocumentMessage: &waProto.DocumentMessage{
		Title:         proto.String(args[2]),
		Caption:       proto.String(caption),
		Url:           proto.String(uploaded.URL),
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		Mimetype:      proto.String(mimeType),
		FileEncSha256: uploaded.FileEncSHA256,
		FileSha256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(data))),
	}}
	resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
	if err != nil {
		c.Logger.Errorf("Error sending document message: %v", err)
	} else {
		c.Logger.Infof("Document message sent (server timestamp: %s)", resp.Timestamp)
	}
	return nil
}

func (c *Client) handleSendVideoCommand(args []string) error {
	if len(args) < 2 {
		c.Logger.Errorf("Usage: sendvid <jid> <video path> [caption]")
		return nil
	}
	recipient, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	}

	data, err := os.ReadFile(args[1])
	if err != nil {
		c.Logger.Errorf("Failed to read %s: %v", args[1], err)
		return nil
	}

	thumbnail, err := createThumbnail(args[1])
	if err != nil {
		c.Logger.Errorf("Error creating thumbnail: %v", err)
		return nil
	}

	uploaded, err := c.WAClient.Upload(context.Background(), data, whatsmeow.MediaVideo)
	if err != nil {
		c.Logger.Errorf("Failed to upload video: %v", err)
		return nil
	}

	msg := &waProto.Message{VideoMessage: &waProto.VideoMessage{
		Caption:       proto.String(strings.Join(args[2:], " ")),
		Url:           proto.String(uploaded.URL),
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		Mimetype:      proto.String(http.DetectContentType(data)),
		FileEncSha256: uploaded.FileEncSHA256,
		FileSha256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(data))),
		JpegThumbnail: thumbnail,
	}}
	resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
	if err != nil {
		c.Logger.Errorf("Error sending video message: %v", err)
	} else {
		c.Logger.Infof("Video message sent (server timestamp: %s)", resp.Timestamp)
	}
	return nil
}

func createThumbnail(videoPath string) ([]byte, error) {
	outBuf := new(bytes.Buffer)
	cmd := exec.Command("ffmpeg", "-y", "-i", videoPath, "-vframes", "1", "-q:v", "2", "-f", "mjpeg", "pipe:1")
	cmd.Stdout = outBuf
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(outBuf)
	if err != nil {
		return nil, err
	}
	thumbnail := resizeImage(img)
	buffer := new(bytes.Buffer)
	err = jpeg.Encode(buffer, thumbnail, nil)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func resizeImage(img image.Image) image.Image {
	maxSize := 100
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width > height {
		if width > maxSize {
			return resize.Resize(uint(maxSize), 0, img, resize.Lanczos3)
		}
	} else {
		if height > maxSize {
			return resize.Resize(0, uint(maxSize), img, resize.Lanczos3)
		}
	}
	return img
}

func (c *Client) handleSendAudioCommand(args []string) error {
	if len(args) < 2 {
		c.Logger.Errorf("Usage: sendaudio <jid> <audio path>")
		return nil
	}
	recipient, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	}

	data, err := os.ReadFile(args[1])
	if err != nil {
		c.Logger.Errorf("Failed to read %s: %v", args[1], err)
		return nil
	}

	uploaded, err := c.WAClient.Upload(context.Background(), data, whatsmeow.MediaAudio)
	if err != nil {
		c.Logger.Errorf("Failed to upload audio: %v", err)
		return nil
	}

	msg := &waProto.Message{AudioMessage: &waProto.AudioMessage{
		Url:           proto.String(uploaded.URL),
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		Mimetype:      proto.String(http.DetectContentType(data)),
		FileEncSha256: uploaded.FileEncSHA256,
		FileSha256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(data))),
	}}
	resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
	if err != nil {
		c.Logger.Errorf("Error sending audio message: %v", err)
	} else {
		c.Logger.Infof("Audio message sent (server timestamp: %s)", resp.Timestamp)
	}
	return nil
}

func (c *Client) handleSendImageCommand(args []string) error {
	if len(args) < 2 {
		c.Logger.Errorf("Usage: sendimg <jid> <image path> [caption]")
		return nil
	}
	recipient, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	}
	data, err := os.ReadFile(args[1])
	if err != nil {
		c.Logger.Errorf("Failed to read %s: %v", args[1], err)
		return nil
	}

	thumbnail, err := createThumbnail(args[1])
	if err != nil {
		c.Logger.Errorf("Error creating thumbnail: %v", err)
		return nil
	}

	uploaded, err := c.WAClient.Upload(context.Background(), data, whatsmeow.MediaImage)
	if err != nil {
		c.Logger.Errorf("Failed to upload image: %v", err)
		return nil
	}

	msg := &waProto.Message{ImageMessage: &waProto.ImageMessage{
		Caption:       proto.String(strings.Join(args[2:], " ")),
		Url:           proto.String(uploaded.URL),
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		Mimetype:      proto.String(http.DetectContentType(data)),
		FileEncSha256: uploaded.FileEncSHA256,
		FileSha256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(data))),
		JpegThumbnail: thumbnail,
	}}
	resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
	if err != nil {
		c.Logger.Errorf("Error sending image message: %v", err)
	} else {
		c.Logger.Infof("Image message sent (server timestamp: %s)", resp.Timestamp)
	}
	return nil
}

func (c *Client) handleReactCommand(args []string) error {
	if len(args) < 3 {
		c.Logger.Errorf("Usage: react <jid> <message ID> <reaction>")
		return nil
	}
	recipient, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	}
	messageID := args[1]
	fromMe := false
	if strings.HasPrefix(messageID, "me:") {
		fromMe = true
		messageID = messageID[len("me:"):]
	}
	reaction := args[2]
	if reaction == "remove" {
		reaction = ""
	}
	msg := &waProto.Message{
		ReactionMessage: &waProto.ReactionMessage{
			Key: &waProto.MessageKey{
				RemoteJid: proto.String(recipient.String()),
				FromMe:    proto.Bool(fromMe),
				Id:        proto.String(messageID),
			},
			Text:              proto.String(reaction),
			SenderTimestampMs: proto.Int64(time.Now().UnixMilli()),
		},
	}
	resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
	if err != nil {
		c.Logger.Errorf("Error sending reaction: %v", err)
	} else {
		c.Logger.Infof("Reaction sent (server timestamp: %s)", resp.Timestamp)
	}
	return nil
}

func (c *Client) handleRevokeCommand(args []string) error {
	if len(args) < 2 {
		c.Logger.Errorf("Usage: revoke <jid> <message ID>")
		return nil
	}
	recipient, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	}
	messageID := args[1]
	msg := c.WAClient.BuildRevocation(recipient, types.EmptyJID, messageID)
	resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
	if err != nil {
		c.Logger.Errorf("Error sending revocation: %v", err)
	} else {
		c.Logger.Infof("Revocation sent (server timestamp: %s)", resp.Timestamp)
	}
	return nil
}

func (c *Client) handleMarkReadCommand(args []string) error {
	if len(args) < 2 {
		c.Logger.Errorf("Usage: markread <jid> <message ID 1> [message ID X]")
		return nil
	}
	recipient, ok := utils.ParseJID(args[0])
	if !ok {
		return nil
	}

	messageIDs := args[1:]

	err := c.WAClient.MarkRead(messageIDs, time.Now(), recipient, types.EmptyJID)
	if err != nil {
		c.Logger.Errorf("Error sending mark as read: %v", err)
	} else {
		c.Logger.Infof("Mark as read sent")
	}
	return nil
}

func (c *Client) handleBatchMessageGroupMembersCommand(args []string) error {
	if len(args) < 2 {
		c.Logger.Errorf("Usage: batchsendgroupmembers <group jid> <text>")
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
		return nil
	}
	for _, participant := range resp.Participants {
		participantJID := participant.JID
		if participantJID == c.WAClient.Store.ID {
			continue
		}
		newArgs := []string{participantJID.String()}
		newArgs = append(newArgs, args[1:]...)
		c.handleSendCommand(newArgs)
	}
	return nil
}
