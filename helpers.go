package main

import (
    "encoding/json"
    "fmt"
    "go.mau.fi/whatsmeow/types"
    "strings"
    "os"
    "path/filepath"
    "crypto/sha256"
    "image"
    "image/jpeg"
    _ "image/png"
    "github.com/zRedShift/mimemagic"
)

// ParseJID parses a string into a types.JID object, handling different input formats.
func ParseJID(arg string) (types.JID, bool) {
    if arg == "" {
        return types.JID{}, false
    }
    if arg[0] == '+' {
        arg = arg[1:]
    }
    if !strings.ContainsRune(arg, '@') {
        return types.NewJID(arg, types.DefaultUserServer), true
    } else {
        recipient, err := types.ParseJID(arg)
        if err != nil {
            return recipient, false
        } else if recipient.User == "" {
            return recipient, false
        }
        return recipient, true
    }
}

func AppendToJSON(initialJSON string, keyword string, data interface{}) (string, error) {
    myJSON, err := FromJSON(initialJSON)
    if err != nil {
        return "", fmt.Errorf("error parsing initial JSON: %w", err)
    }

    myJSON[keyword] = data

    jsonString, err := ToJSON(myJSON)
    if err != nil {
        return "", fmt.Errorf("error converting to JSON: %w", err)
    }

    return jsonString, nil
}

func FromJSON(jsonString string) (map[string]interface{}, error) {
    var jsonData map[string]interface{}
    err := json.Unmarshal([]byte(jsonString), &jsonData)
    if err != nil {
        return nil, err
    }
    return jsonData, nil
}

func ToJSON(jsonData map[string]interface{}) (string, error) {
    jsonBytes, err := json.Marshal(jsonData)
    if err != nil {
        return "", err
    }
    return string(jsonBytes), nil
}

func Max(a, b int) int {
    if a > b {
        return a
    }
    return b
}

func ResizeImage(img image.Image) image.Image {
    width := img.Bounds().Dx()
    height := img.Bounds().Dy()
    newWidth := width
    newHeight := height
    maxSize := 299

    if !(width <= maxSize && height <= maxSize) {
        scaleFactor := float64(maxSize) / float64(Max(width, height))
        newWidth = int(float64(width) * scaleFactor)
        newHeight = int(float64(height) * scaleFactor)
    }

    thumbnail := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
    originalBounds := img.Bounds()
    scaleX := float64(originalBounds.Dx()) / float64(newWidth)
    scaleY := float64(originalBounds.Dy()) / float64(newHeight)

    for x := 0; x < newWidth; x++ {
        for y := 0; y < newHeight; y++ {
            px := int(float64(x) * scaleX)
            py := int(float64(y) * scaleY)
            thumbnail.Set(x, y, img.At(px, py))
        }
    }

    return thumbnail
}

func SavePollQuestionAndOptions(messageID string, question string, options []string, baseDir string) error {
    err := os.MkdirAll(filepath.Join(baseDir, ".tmp"), os.ModePerm)
    if err != nil {
        return fmt.Errorf("failed to create directory: %w", err)
    }
    err := os.WriteFile(filepath.Join(baseDir, ".tmp", "poll_question_"+messageID), []byte(question), 0644)
    if err != nil {
        return fmt.Errorf("failed to save poll question: %w", err)
    }

    for _, option := range options {
        sha := fmt.Sprintf("%x", sha256.Sum256([]byte(option)))
        err := os.WriteFile(filepath.Join(baseDir, ".tmp", "poll_option_"+sha), []byte(option), 0644)
        if err != nil {
            return fmt.Errorf("failed to save poll option: %w", err)
        }
    }

    return nil
}

func MatchMimeType(data []byte) *mimemagic.MatchResult {
    return mimemagic.MatchMagic(data)
}
