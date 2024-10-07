package whatsapp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io/ioutil"
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
	waCommon "go.mau.fi/whatsmeow/types/events"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	waProto "go.mau.fi/whatsmeow/proto/waProto"
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
	if recipient.Server == types.GroupServer {
		msg.MessageContextInfo = &waProto.MessageContextInfo{MessageSecret: random.Bytes(32)}
	}
	resp, err := c.WAClient.SendMessage(context.Background(), recipient, "", msg)
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

	itemTmp := ""
	for i := 0; i < len(items); i++ {
		if (i+1)%3 == 0 {
			if items[i] != "/" {
				c.Logger.Errorf("Error at \"%s\"", items[i])
				c.Logger.Errorf("Separator \"/\" is missing")
				return nil
			} else if len(items) != 2 && i+1 == len(items) {
				c.Logger.Errorf("Missing items after \"/\"")
				return nil
			}
		} else if i%3 == 0 {
			itemTmp = items[i]
			if i+1 == len(items) {
				c.Logger.Errorf("Error at \"%s\"", items[i])
				c.Logger.Errorf("Missing description after heading")
				return nil
			}
		} else if (i+2)%3 == 0 {
			newRow := &waProto.ListMessage_Row{
				RowId:       proto.String(fmt.Sprintf("id%d", i+1)),
				Title:       proto.String(itemTmp),
				Description: proto.String(items[i]),
			}
			msg.ListMessage.Sections[0].Rows = append(msg.ListMessage.Sections[0].Rows, newRow)
			if i+1 == len(items) {
				break
			}
		}
	}
	if recipient.Server == types.GroupServer {
		msg.MessageContextInfo = &waProto.MessageContextInfo{MessageSecret: random.Bytes(32)}
	}
	resp, err := c.WAClient.SendMessage(context.Background(), recipient, "", msg)
	if err != nil {
		c.Logger.Errorf("Error sending message: %v", err)
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

	if c.Config.Mode == "both" {
		os.MkdirAll(filepath.Join(c.CurrentDir, ".tmp"), os.ModePerm)
		msgID := whatsmeow.GenerateMessageID()
		err := os.WriteFile(filepath.Join(c.CurrentDir, ".tmp", "poll_question_"+msgID), []byte(question), 0644)
		if err != nil {
			c.Logger.Errorf("Failed to save poll question: %v", err)
			return nil
		}

		for _, option := range options {
			sha := fmt.Sprintf("%x", sha256.Sum256([]byte(option)))
			err := os.WriteFile(filepath.Join(c.CurrentDir, ".tmp", "poll_option_"+sha), []byte(option), 0644)
			if err != nil {
				c.Logger.Errorf("Failed to save poll option: %v", err)
				return nil
			}
		}
		resp, err := c.WAClient.SendMessage(context.Background(), recipient, msgID, c.WAClient.BuildPollCreation(question, options, maxAnswers))
		if err != nil {
			c.Logger.Errorf("Error sending message: %v", err)
		} else {
			c.Logger.Infof("Poll message sent (server timestamp: %s)", resp.Timestamp)
		}
		return nil
	}

	resp, err := c.WAClient.SendMessage(context.Background(), recipient, "", c.WAClient.BuildPollCreation(question, options, maxAnswers))
	if err != nil {
		c.Logger.Errorf("Error sending message: %v", err)
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
	if err != nil {
		c.Logger.Errorf("Could not fetch Open Graph data: %s", err)
		msg := &waProto.Message{ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text:         proto.String(args[1] + text),
			CanonicalUrl: proto.String(args[1]),
			MatchedText:  proto.String(args[1]),
		}}
		if recipient.Server == types.GroupServer {
			msg.MessageContextInfo = &waProto.MessageContextInfo{MessageSecret: random.Bytes(32)}
		}
		resp, err := c.WAClient.SendMessage(context.Background(), recipient, "", msg)
		if err != nil {
			c.Logger.Errorf("Error sending link message: %v", err)
		} else {
			c.Logger.Infof("Link message sent (server timestamp: %s)", resp.Timestamp)
		}
		return nil
	}

	ogp.ToAbs()

	if ogp.Title == "" || ogp.Description == "" || len(ogp.Image) == 0 || ogp.Image[0].URL == "" {
		c.Logger.Errorf("Could not fetch Open Graph data: Missing Open Graph content")
		msg := &waProto.Message{ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text:         proto.String(args[1] + text),
			CanonicalUrl: proto.String(args[1]),
			MatchedText:  proto.String(args[1]),
		}}
		if recipient.Server == types.GroupServer {
			msg.MessageContextInfo = &waProto.MessageContextInfo{MessageSecret: random.Bytes(32)}
		}
		resp, err := c.WAClient.SendMessage(context.Background(), recipient, "", msg)
		if err != nil {
			c.Logger.Errorf("Error sending link message: %v", err)
		} else {
			c.Logger.Infof("Link message sent (server timestamp: %s)", resp.Timestamp)
		}
		return nil
	}

	data, err := http.Get(ogp.Image[0].URL)
	if err != nil {
		c.Logger.Errorf("Could not fetch thumbnail data: %s", err)
		return nil
	}

	if data.StatusCode != http.StatusOK {
		c.Logger.Errorf("Could not fetch thumbnail data: %d", data.StatusCode)
		return nil
	}

	jpegBytes, err := ioutil.ReadAll(data.Body)
	if err != nil {
		c.Logger.Errorf("Could not fetch thumbnail data: %s", err)
		return nil
	}
	data.Body.Close()

	config, _, err := image.DecodeConfig(bytes.NewReader(jpegBytes))
	if err != nil {
		c.Logger.Errorf("Could not decode image: %s", err)
		err = nil
		var thumbnailResp whatsmeow.UploadResponse
		if recipient.Server == types.NewsletterServer {
			thumbnailResp, err = c.WAClient.UploadNewsletter(context.Background(), jpegBytes, whatsmeow.MediaLinkThumbnail)
		} else {
			thumbnailResp, err = c.WAClient.Upload(context.Background(), jpegBytes, whatsmeow.MediaLinkThumbnail)
		}
		if err != nil {
			c.Logger.Errorf("Failed to upload preview thumbnail file: %v", err)
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
			MediaKey:            thumbnailResp.MediaKey,
		}}
		if recipient.Server == types.GroupServer {
			msg.MessageContextInfo = &waProto.MessageContextInfo{MessageSecret: random.Bytes(32)}
		}
		resp, err := c.WAClient.SendMessage(context.Background(), recipient, "", msg)
		if err != nil {
			c.Logger.Errorf("Error sending link message: %v", err)
		} else {
			c.Logger.Infof("Link message sent (server timestamp: %s)", resp.Timestamp)
		}
	} else {
		var thumbnailResp whatsmeow.UploadResponse
		if recipient.Server == types.NewsletterServer {
			thumbnailResp, err = c.WAClient.UploadNewsletter(context.Background(), jpegBytes, whatsmeow.MediaLinkThumbnail)
		} else {
			thumbnailResp, err = c.WAClient.Upload(context.Background(), jpegBytes, whatsmeow.MediaLinkThumbnail)
		}
		if err != nil {
			c.Logger.Errorf("Failed to upload preview thumbnail file: %v", err)
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
		if recipient.Server == types.GroupServer {
			msg.MessageContextInfo = &waProto.MessageContextInfo{MessageSecret: random.Bytes(32)}
		}
		resp, err := c.WAClient.SendMessage(context.Background(), recipient, "", msg)
		if err != nil {
			c.Logger.Errorf("Error sending link message: %v", err)
		} else {
			c.Logger.Infof("Link message sent (server timestamp: %s)", resp.Timestamp)
		}
	}
	return nil
}

func (c *Client) handleSendDocumentCommand(args []string) error {
    if len(args) < 3 {
        c.Logger.Errorf("Usage: senddoc <jid> <document path> <document file name> [caption] [mime-type]")
        return
    }
    recipient, ok := utils.ParseJID(args[0])
    if !ok {
        return
    }
    data, err := os.ReadFile(args[1])
    if err != nil {
        c.Logger.Errorf("Failed to read %s: %v", args[1], err)
        return
    }
    uploaded := whatsmeow.UploadResponse{}
    if recipient.Server == types.NewsletterServer {
        uploaded, err = c.WAClient.UploadNewsletter(context.Background(), data, whatsmeow.MediaDocument)
    } else {
        uploaded, err = c.WAClient.Upload(context.Background(), data, whatsmeow.MediaDocument)
    }
    if err != nil {
        c.Logger.Errorf("Failed to upload file: %v", err)
        return
    }
    caption := ""
    if len(args) > 3 && args[3] != "" {
        caption = args[3]
    }
    if len(args) < 5 {
        msg := &waProto.Message{DocumentMessage: &waProto.DocumentMessage{
            Title:         proto.String(args[2]),
            Caption:       proto.String(caption),
            Url:           proto.String(uploaded.URL),
            DirectPath:    proto.String(uploaded.DirectPath),
            MediaKey:      uploaded.MediaKey,
            Mimetype:      proto.String(http.DetectContentType(data)),
            FileEncSha256: uploaded.FileEncSHA256,
            FileSha256:    uploaded.FileSHA256,
            FileLength:    proto.Uint64(uint64(len(data))),
        }}
        if recipient.Server == types.GroupServer {
            msg.MessageContextInfo = &waProto.MessageContextInfo{MessageSecret: random.Bytes(32)}
        }
        resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
        if err != nil {
            c.Logger.Errorf("Error sending document message: %v", err)
        } else {
            c.Logger.Infof("Document message sent (server timestamp: %s)", resp.Timestamp)
        }
    } else {
        msg := &waProto.Message{DocumentMessage: &waProto.DocumentMessage{
            Title:         proto.String(args[2]),
            Caption:       proto.String(caption),
            Url:           proto.String(uploaded.URL),
            DirectPath:    proto.String(uploaded.DirectPath),
            MediaKey:      uploaded.MediaKey,
            Mimetype:      proto.String(args[4]),
            FileEncSha256: uploaded.FileEncSHA256,
            FileSha256:    uploaded.FileSHA256,
            FileLength:    proto.Uint64(uint64(len(data))),
        }}
        if recipient.Server == types.GroupServer {
            msg.MessageContextInfo = &waProto.MessageContextInfo{MessageSecret: random.Bytes(32)}
        }
        resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
        if err != nil {
            c.Logger.Errorf("Error sending document message: %v", err)
        } else {
            c.Logger.Infof("Document message sent (server timestamp: %s)", resp.Timestamp)
        }
    }
}

func (c *Client) handleSendVideoCommand(args []string) error {
    if len(args) < 2 {
        c.Logger.Errorf("Usage: sendvid <jid> <video path> [caption]")
        return
    }
    recipient, ok := utils.ParseJID(args[0])
    if !ok {
        return
    }
    
    data, err := os.ReadFile(args[1])
    if err != nil {
        c.Logger.Errorf("Failed to read %s: %v", args[1], err)
        return
    }
    
    outBuf := new(bytes.Buffer)
    
    command := []string{
        c.FFmpegScriptPath,
        "-y",
        "-i", args[1],
        "-hide_banner",
        "-nostats",
        "-loglevel", "0",
        "-vframes", "1",
        "-q:v", "0",
        "-f", "mjpeg",
        "pipe:1",
    }
    
    cmd := exec.Command("sh", command...)
    cmd.Stdout = outBuf
    
    err = cmd.Run()
    if err != nil {
        c.Logger.Errorf("Error while using ffmpeg to create thumbnail: %s", err)
        c.Logger.Errorf("Sending video without preview thumbnail")
        err := error(nil)
        uploaded := whatsmeow.UploadResponse{}
        if recipient.Server == types.NewsletterServer {
            uploaded, err = c.WAClient.UploadNewsletter(context.Background(), data, whatsmeow.MediaVideo)
        } else {
            uploaded, err = c.WAClient.Upload(context.Background(), data, whatsmeow.MediaVideo)
        }
        if err != nil {
            c.Logger.Errorf("Failed to upload file: %v", err)
            return
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
        }}
        if recipient.Server == types.GroupServer {
            msg.MessageContextInfo = &waProto.MessageContextInfo{MessageSecret: random.Bytes(32)}
        }
        resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
        if err != nil {
            c.Logger.Errorf("Error sending video message: %v", err)
        } else {
            c.Logger.Infof("Video message sent (server timestamp: %s)", resp.Timestamp)
        }
        return
    }
    
    img, _, err := image.Decode(outBuf)
    if err != nil {
        c.Logger.Errorf("Error decoding image: %s", err)
        return
    }
    
    thumbnail := resizeImage(img)
    
    buffer := new(bytes.Buffer)
    
    err = jpeg.Encode(buffer, thumbnail, nil)
    if err != nil {
        c.Logger.Errorf("Error encoding thumbnail: %s", err)
        return
    }
    
    jpegBytes := buffer.Bytes()
    
    uploaded := whatsmeow.UploadResponse{}
    if recipient.Server == types.NewsletterServer {
        uploaded, err = c.WAClient.UploadNewsletter(context.Background(), data, whatsmeow.MediaVideo)
    } else {
        uploaded, err = c.WAClient.Upload(context.Background(), data, whatsmeow.MediaVideo)
    }
    if err != nil {
        c.Logger.Errorf("Failed to upload file: %v", err)
        return
    }
    thumbnailResp := whatsmeow.UploadResponse{}
    if recipient.Server == types.NewsletterServer {
        thumbnailResp, err = c.WAClient.UploadNewsletter(context.Background(), jpegBytes, whatsmeow.MediaImage)
    } else {
        thumbnailResp, err = c.WAClient.Upload(context.Background(), jpegBytes, whatsmeow.MediaImage)
    }
    if err != nil {
        c.Logger.Errorf("Failed to upload preview thumbnail file: %v", err)
        return
    }
    
    msg := &waProto.Message{VideoMessage: &waProto.VideoMessage{
        Caption:       proto.String(strings.Join(args[2:], " ")),
        Url:           proto.String(uploaded.URL),
        DirectPath:    proto.String(uploaded.DirectPath),
        ThumbnailDirectPath: &thumbnailResp.DirectPath,
        ThumbnailSha256: thumbnailResp.FileSHA256,
        ThumbnailEncSha256: thumbnailResp.FileEncSHA256,
        JpegThumbnail: jpegBytes,
        MediaKey:      uploaded.MediaKey,
        Mimetype:      proto.String(http.DetectContentType(data)),
        FileEncSha256: uploaded.FileEncSHA256,
        FileSha256:    uploaded.FileSHA256,
        FileLength:    proto.Uint64(uint64(len(data))),
    }}
    if recipient.Server == types.GroupServer {
        msg.MessageContextInfo = &waProto.MessageContextInfo{MessageSecret: random.Bytes(32)}
    }
    resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
    if err != nil {
        c.Logger.Errorf("Error sending video message: %v", err)
    } else {
        c.Logger.Infof("Video message sent (server timestamp: %s)", resp.Timestamp)
    }
}

func (c *Client) handleSendAudioCommand(args []string) error {
    if len(args) < 2 {
        c.Logger.Errorf("Usage: sendaudio <jid> <audio path>")
        return
    }
    recipient, ok := utils.ParseJID(args[0])
    if !ok {
        return
    }
    
    outBuf := new(bytes.Buffer)
    
    command := []string{
        c.FFmpegScriptPath,
        "-y",
        "-i", args[1],
        "-hide_banner",
        "-nostats",
        "-loglevel", "0",
        "-codec:a", "libopus",
        "-ac", "1",
        "-ar", "48000",
        "-f", "ogg",
        "pipe:1",
    }
    
    cmd := exec.Command("sh", command...)
    cmd.Stdout = outBuf
    
    err := cmd.Run()
    if err != nil {
        c.Logger.Errorf("Error while using ffmpeg to fix audio: %s", err)
        c.Logger.Errorf("Sending raw and unfixed audio")
        data, err := os.ReadFile(args[1])
        if err != nil {
            c.Logger.Errorf("Failed to read %s: %v", args[1], err)
            return
        }
        uploaded := whatsmeow.UploadResponse{}
        if recipient.Server == types.NewsletterServer {
            uploaded, err = c.WAClient.UploadNewsletter(context.Background(), data, whatsmeow.MediaAudio)
        } else {
            uploaded, err = c.WAClient.Upload(context.Background(), data, whatsmeow.MediaAudio)
        }
        if err != nil {
            c.Logger.Errorf("Failed to upload file: %v", err)
            return
        }
        
        msg := &waProto.Message{AudioMessage: &waProto.AudioMessage{
            Url:           proto.String(uploaded.URL),
            DirectPath:    proto.String(uploaded.DirectPath),
            MediaKey:      uploaded.MediaKey,
            Mimetype:      proto.String("audio/ogg; codecs=opus"),
            FileEncSha256: uploaded.FileEncSHA256,
            FileSha256:    uploaded.FileSHA256,
            FileLength:    proto.Uint64(uint64(len(data))),
        }}
        if recipient.Server == types.GroupServer {
            msg.MessageContextInfo = &waProto.MessageContextInfo{MessageSecret: random.Bytes(32)}
        }
        resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
        if err != nil {
            c.Logger.Errorf("Error sending audio message: %v", err)
        } else {
            c.Logger.Infof("Audio message sent (server timestamp: %s)", resp.Timestamp)
        }
        return
    }
    
    data := outBuf.Bytes()
    
    err = nil
    uploaded := whatsmeow.UploadResponse{}
    if recipient.Server == types.NewsletterServer {
        uploaded, err = c.WAClient.UploadNewsletter(context.Background(), data, whatsmeow.MediaAudio)
    } else {
        uploaded, err = c.WAClient.Upload(context.Background(), data, whatsmeow.MediaAudio)
    }
    if err != nil {
        c.Logger.Errorf("Failed to upload file: %v", err)
        return
    }
    
    msg := &waProto.Message{AudioMessage: &waProto.AudioMessage{
        Url:           proto.String(uploaded.URL),
        DirectPath:    proto.String(uploaded.DirectPath),
        MediaKey:      uploaded.MediaKey,
        Mimetype:      proto.String("audio/ogg; codecs=opus"),
        FileEncSha256: uploaded.FileEncSHA256,
        FileSha256:    uploaded.FileSHA256,
        FileLength:    proto.Uint64(uint64(len(data))),
    }}
    if recipient.Server == types.GroupServer {
        msg.MessageContextInfo = &waProto.MessageContextInfo{MessageSecret: random.Bytes(32)}
    }
    resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
    if err != nil {
        c.Logger.Errorf("Error sending audio message: %v", err)
    } else {
        c.Logger.Infof("Audio message sent (server timestamp: %s)", resp.Timestamp)
    }
}

func (c *Client) handleSendImageCommand(args []string) error {
    if len(args) < 2 {
        c.Logger.Errorf("Usage: sendimg <jid> <image path> [caption]")
        return
    }
    recipient, ok := utils.ParseJID(args[0])
    if !ok {
        return
    }
    data, err := os.ReadFile(args[1])
    if err != nil {
        c.Logger.Errorf("Failed to read %s: %v", args[1], err)
        return
    }
    
    isCompatible := false
    
    mimeType := mimemagic.MatchMagic(data)
    
    if len(mimeType.Extensions) != 0 {
        compatibleFormats := []string{".jpg", ".jpeg", ".jpe", ".png"}
        joinedFormats := strings.Join(compatibleFormats, "|")
        isCompatible = strings.Contains(strings.Join(mimeType.Extensions, "|"), joinedFormats)
    }
    
    outBuf := new(bytes.Buffer)
    
    command := []string{
        c.FFmpegScriptPath,
        "-y",
        "-i", args[1],
        "-hide_banner",
        "-nostats",
        "-loglevel", "0",
        "-vframes", "1",
        "-q:v", "0",
        "-f", "mjpeg",
        "pipe:1",
    }
    
    cmd := exec.Command("sh", command...)
    cmd.Stdout = outBuf
    
    err = cmd.Run()
    if err != nil {
        c.Logger.Errorf("Error while using ffmpeg to create thumbnail: %s", err)
        c.Logger.Infof("Using fallback method to generate thumbnail")
        imageFile, err := os.Open(args[1])
        if err != nil {
            c.Logger.Errorf("Error opening image file: %s", err)
            return
        }
        img, _, err := image.Decode(imageFile)
        if err != nil {
            c.Logger.Errorf("Error decoding image: %s", err)
            c.Logger.Errorf("Sending image without preview thumbnail")
            err := error(nil)
            uploaded := whatsmeow.UploadResponse{}
            if recipient.Server == types.NewsletterServer {
                uploaded, err = c.WAClient.UploadNewsletter(context.Background(), data, whatsmeow.MediaImage)
            } else {
                uploaded, err = c.WAClient.Upload(context.Background(), data, whatsmeow.MediaImage)
            }
            if err != nil {
                c.Logger.Errorf("Failed to upload file: %v", err)
                return
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
            }}
            if recipient.Server == types.GroupServer {
                msg.MessageContextInfo = &waProto.MessageContextInfo{MessageSecret: random.Bytes(32)}
            }
            resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
            if err != nil {
                c.Logger.Errorf("Error sending image message: %v", err)
            } else {
                c.Logger.Infof("Image message sent (server timestamp: %s)", resp.Timestamp)
            }
            return
        }
        imageFile.Close()
        
        thumbnail := resizeImage(img)
        
        buffer := new(bytes.Buffer)
        
        err = jpeg.Encode(buffer, thumbnail, nil)
        if err != nil {
            c.Logger.Errorf("Error encoding thumbnail: %s", err)
            return
        }
        
        jpegBytes := buffer.Bytes()
        
        uploaded := whatsmeow.UploadResponse{}
        if recipient.Server == types.NewsletterServer {
            uploaded, err = c.WAClient.UploadNewsletter(context.Background(), data, whatsmeow.MediaImage)
        } else {
            uploaded, err = c.WAClient.Upload(context.Background(), data, whatsmeow.MediaImage)
        }
        if err != nil {
            c.Logger.Errorf("Failed to upload file: %v", err)
            return
        }
        thumbnailResp := whatsmeow.UploadResponse{}
        if recipient.Server == types.NewsletterServer {
            thumbnailResp, err = c.WAClient.UploadNewsletter(context.Background(), jpegBytes, whatsmeow.MediaImage)
        } else {
            thumbnailResp, err = c.WAClient.Upload(context.Background(), jpegBytes, whatsmeow.MediaImage)
        }
        if err != nil {
            c.Logger.Errorf("Failed to upload preview thumbnail file: %v", err)
            return
        }
        msg := &waProto.Message{ImageMessage: &waProto.ImageMessage{
            Caption:       proto.String(strings.Join(args[2:], " ")),
            Url:           proto.String(uploaded.URL),
            DirectPath:    proto.String(uploaded.DirectPath),
            ThumbnailDirectPath: &thumbnailResp.DirectPath,
            ThumbnailSha256: thumbnailResp.FileSHA256,
            ThumbnailEncSha256: thumbnailResp.FileEncSHA256,
            JpegThumbnail: jpegBytes,
            MediaKey:      uploaded.MediaKey,
            Mimetype:      proto.String(http.DetectContentType(data)),
            FileEncSha256: uploaded.FileEncSHA256,
            FileSha256:    uploaded.FileSHA256,
            FileLength:    proto.Uint64(uint64(len(data))),
        }}
        if recipient.Server == types.GroupServer {
            msg.MessageContextInfo = &waProto.MessageContextInfo{MessageSecret: random.Bytes(32)}
        }
        if recipient.Server == types.GroupServer {
            msg.MessageContextInfo = &waProto.MessageContextInfo{MessageSecret: random.Bytes(32)}
        }
        resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
        if err != nil {
            c.Logger.Errorf("Error sending image message: %v", err)
        } else {
            c.Logger.Infof("Image message sent (server timestamp: %s)", resp.Timestamp)
        }
        
        return
    }
    
    outBytes := outBuf.Bytes()
    img, _, err := image.Decode(outBuf)
    if err != nil {
        c.Logger.Errorf("Error decoding image: %s", err)
        return
    }
    
    thumbnail := resizeImage(img)
    
    buffer := new(bytes.Buffer)
    
    err = jpeg.Encode(buffer, thumbnail, nil)
    if err != nil {
        c.Logger.Errorf("Error encoding thumbnail: %s", err)
        return
    }
    
    jpegBytes := buffer.Bytes()
    
    uploaded := whatsmeow.UploadResponse{}
    lenData := len(data)
    contentData := data
    if isCompatible {
        err := error(nil)
        if recipient.Server == types.NewsletterServer {
            uploaded, err = c.WAClient.UploadNewsletter(context.Background(), data, whatsmeow.MediaImage)
        } else {
            uploaded, err = c.WAClient.Upload(context.Background(), data, whatsmeow.MediaImage)
        }
        if err != nil {
            c.Logger.Errorf("Failed to upload file: %v", err)
            return
        }
    } else {
        lenData = len(outBytes)
        contentData = outBytes
        err := error(nil)
        if recipient.Server == types.NewsletterServer {
            uploaded, err = c.WAClient.UploadNewsletter(context.Background(), outBytes, whatsmeow.MediaImage)
        } else {
            uploaded, err = c.WAClient.Upload(context.Background(), outBytes, whatsmeow.MediaImage)
        }
        if err != nil {
            c.Logger.Errorf("Failed to upload file: %v", err)
            return
        }
    }
    err = nil
    thumbnailResp := whatsmeow.UploadResponse{}
    if recipient.Server == types.NewsletterServer {
        thumbnailResp, err = c.WAClient.UploadNewsletter(context.Background(), jpegBytes, whatsmeow.MediaImage)
    } else {
        thumbnailResp, err = c.WAClient.Upload(context.Background(), jpegBytes, whatsmeow.MediaImage)
    }
    if err != nil {
        c.Logger.Errorf("Failed to upload preview thumbnail file: %v", err)
        return
    }
    msg := &waProto.Message{ImageMessage: &waProto.ImageMessage{
        Caption:       proto.String(strings.Join(args[2:], " ")),
        Url:           proto.String(uploaded.URL),
        DirectPath:    proto.String(uploaded.DirectPath),
        ThumbnailDirectPath: &thumbnailResp.DirectPath,
        ThumbnailSha256: thumbnailResp.FileSHA256,
        ThumbnailEncSha256: thumbnailResp.FileEncSHA256,
        JpegThumbnail: jpegBytes,
        MediaKey:      uploaded.MediaKey,
        Mimetype:      proto.String(http.DetectContentType(contentData)),
        FileEncSha256: uploaded.FileEncSHA256,
        FileSha256:    uploaded.FileSHA256,
        FileLength:    proto.Uint64(uint64(lenData)),
    }}
    if recipient.Server == types.GroupServer {
        msg.MessageContextInfo = &waProto.MessageContextInfo{MessageSecret: random.Bytes(32)}
    }
    resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
    if err != nil {
        c.Logger.Errorf("Error sending image message: %v", err)
    } else {
        c.Logger.Infof("Image message sent (server timestamp: %s)", resp.Timestamp)
    }
}

func (c *Client) handleReactCommand(args []string) error {
    if len(args) < 3 {
        c.Logger.Errorf("Usage: react <jid> <message ID> <reaction>")
        return
    }
    recipient, ok := utils.ParseJID(args[0])
    if !ok {
        return
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
    msg := &waE2E.Message{
        ReactionMessage: &waE2E.ReactionMessage{
            Key: &waCommon.MessageKey{
                RemoteJID: proto.String(recipient.String()),
                FromMe:    proto.Bool(fromMe),
                ID:        proto.String(messageID),
            },
            Text:              proto.String(reaction),
            SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
        },
    }
    if recipient.Server == types.GroupServer {
        msg.MessageContextInfo = &waProto.MessageContextInfo{MessageSecret: random.Bytes(32)}
    }
    resp, err := c.WAClient.SendMessage(context.Background(), recipient, msg)
    if err != nil {
        c.Logger.Errorf("Error sending reaction: %v", err)
    } else {
        c.Logger.Infof("Reaction sent (server timestamp: %s)", resp.Timestamp)
    }
}

func (c *Client) handleRevokeCommand(args []string) error {
    if len(args) < 2 {
        c.Logger.Errorf("Usage: revoke <jid> <message ID>")
        return
    }
    recipient, ok := utils.ParseJID(args[0])
    if !ok {
        return
    }
    messageID := args[1]
    resp, err := c.WAClient.SendMessage(context.Background(), recipient, cli.BuildRevoke(recipient, types.EmptyJID, messageID))
    if err != nil {
        c.Logger.Errorf("Error sending revocation: %v", err)
    } else {
        c.Logger.Infof("Revocation sent (server timestamp: %s)", resp.Timestamp)
    }
}

func (c *Client) handleMarkReadCommand(args []string) error {
    if len(args) < 2 {
        c.Logger.Errorf("Usage: markread <jid> <message ID 1> [message ID X] (Note: Can add multiple message IDs to mark as read. [] is optional)")
        return
    }
    recipient, ok := utils.ParseJID(args[0])
    if !ok {
        return
    }
    
    messageID := make([]string, 0, len(args)-1)
    for _, id := range args[1:] {
        if id != "" {
            messageID = append(messageID, id)
        }
    }
    
    err := c.WAClient.MarkRead(messageID, time.Now(), recipient, types.EmptyJID)
    if err != nil {
        c.Logger.Errorf("Error sending mark as read: %v", err)
    } else {
        c.Logger.Infof("Mark as read sent")
    }
}

func (c *Client) handleBatchMessageGroupMembersCommand(args []string) error {
    if len(args) < 2 {
        c.Logger.Errorf("Usage: batchsendgroupmembers <group jid> <text>")
        return
    }
    group, ok := utils.ParseJID(args[0])
    if !ok {
        return
    } else if group.Server != types.GroupServer {
        c.Logger.Errorf("Input must be a group JID (@%s)", types.GroupServer)
        c.Logger.Errorf("Usage: batchsendgroupmembers send <group jid> <text>")
        return
    }
    resp, err := c.WAClient.GetGroupInfo(group)
    if err != nil {
        c.Logger.Errorf("Failed to get group info: %v", err)
    } else {
        for _, participant := range resp.Participants {
            participant_jid := fmt.Sprintf("%s", participant.JID)
            if participant_jid == default_jid {
                c.Logger.Infof("skipped messaging self")
            } else {
                new_args := []string{}
                new_args = append(new_args, participant_jid)
                new_args = append(new_args, args[1:]...)
                handleCmd("send", new_args[0:])
            }
        }
    }
}

