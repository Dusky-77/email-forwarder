package gmail

import (
	"encoding/base64"
	"fmt"
	"strings"

	gmailapi "google.golang.org/api/gmail/v1"
)

// Message is our own simplified view of an email, only the fields we actually use
type Message struct {
	ID          string
	SenderEmail string
	SenderName  string
	Subject     string
	Body        string
	ReceivedUnix int64
}

// FetchNew grabs everything that happened since lastHistoryID
// if lastHistoryID is 0 (first run for this account) it just grabs the profile's
// current historyId and returns no messages, so we dont dump the whole inbox on first start
func FetchNew(svc *gmailapi.Service, lastHistoryID uint64) ([]Message, uint64, error) {
	profile, err := svc.Users.GetProfile("me").Do()
	if err != nil {
		return nil, 0, fmt.Errorf("getting profile: %w", err)
	}

	if lastHistoryID == 0 {
		return nil, profile.HistoryId, nil
	}

	var messages []Message
	newestHistoryID := lastHistoryID

	call := svc.Users.History.List("me").StartHistoryId(lastHistoryID).HistoryTypes("messageAdded")

	err = call.Pages(nil, func(page *gmailapi.ListHistoryResponse) error {
		for _, h := range page.History {
			if h.Id > newestHistoryID {
				newestHistoryID = h.Id
			}

			for _, added := range h.MessagesAdded {
				msg, err := fetchMessage(svc, added.Message.Id)
				if err != nil {
					// dont kill the whole batch over one bad message, just skip it
					continue
				}
				messages = append(messages, msg)
			}
		}
		return nil
	})

	if err != nil {
		// history id can expire if its too old, gmail returns 404 in that case
		// caller should treat this as "start fresh from current historyId"
		return nil, 0, fmt.Errorf("listing history: %w", err)
	}

	return messages, newestHistoryID, nil
}

func fetchMessage(svc *gmailapi.Service, id string) (Message, error) {
	full, err := svc.Users.Messages.Get("me", id).Format("full").Do()
	if err != nil {
		return Message{}, err
	}

	msg := Message{
		ID:           id,
		ReceivedUnix: full.InternalDate / 1000, // gmail gives ms, we want seconds
	}

	for _, header := range full.Payload.Headers {
		switch header.Name {
		case "Subject":
			msg.Subject = header.Value
		case "From":
			msg.SenderName, msg.SenderEmail = parseFromHeader(header.Value)
		}
	}

	msg.Body = extractBody(full.Payload)

	return msg, nil
}

// parseFromHeader turns 'John Doe <john@example.com>' into name + email separately
// also handles the case where its just a bare email with no display name
func parseFromHeader(from string) (name string, email string) {
	from = strings.TrimSpace(from)

	start := strings.LastIndex(from, "<")
	end := strings.LastIndex(from, ">")

	if start == -1 || end == -1 || end < start {
		// no angle brackets, whole thing is probably just the email
		return "", strings.TrimSpace(from)
	}

	email = strings.TrimSpace(from[start+1 : end])
	name = strings.TrimSpace(from[:start])
	name = strings.Trim(name, `"`)

	return name, email
}

// extractBody walks the mime parts looking for text/plain, falling back to text/html if thats all there is
func extractBody(part *gmailapi.MessagePart) string {
	if part == nil {
		return ""
	}

	if part.MimeType == "text/plain" && part.Body != nil && part.Body.Data != "" {
		decoded, err := decodeBase64URL(part.Body.Data)
		if err == nil {
			return decoded
		}
	}

	var htmlFallback string

	for _, sub := range part.Parts {
		result := extractBody(sub)
		if result == "" {
			continue
		}
		if sub.MimeType == "text/html" {
			htmlFallback = result
			continue
		}
		return result
	}

	return htmlFallback
}

func decodeBase64URL(data string) (string, error) {
	decoded, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(data)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}
