package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"email-forwarder/gmail"
	"email-forwarder/matcher"
)

type embed struct {
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Color       int          `json:"color"`
	Fields      []embedField `json:"fields"`
	Footer      embedFooter  `json:"footer"`
}

type embedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type embedFooter struct {
	Text string `json:"text"`
}

type webhookPayload struct {
	Content string  `json:"content"`
	Embeds  []embed `json:"embeds"`
}

// PingTarget mirrors config.PingTarget but lives here so this package
// doesnt need to import main (would cause an import cycle)
type PingTarget struct {
	UserIDs []string
	RoleIDs []string
}

// buildMentionString turns a ping target into the raw mention text discord expects
// eg <@123> <@456> <@&789>, users first then roles, doesnt really matter which order
func buildMentionString(ping PingTarget) string {
	var mentions []string

	for _, id := range ping.UserIDs {
		mentions = append(mentions, fmt.Sprintf("<@%s>", id))
	}
	for _, id := range ping.RoleIDs {
		mentions = append(mentions, fmt.Sprintf("<@&%s>", id))
	}

	return strings.Join(mentions, " ")
}

// matchSummary turns the matcher result into a short human readable string for the embed
func matchSummary(m matcher.MatchResult) string {
	var parts []string

	if m.MatchedSenderEmail {
		parts = append(parts, "sender email")
	}
	if m.MatchedSenderDomain {
		parts = append(parts, "sender domain")
	}
	if m.MatchedSenderName {
		parts = append(parts, "sender name")
	}
	if len(m.MatchedSubjectKeywords) > 0 {
		parts = append(parts, fmt.Sprintf("subject keyword (%s)", strings.Join(m.MatchedSubjectKeywords, ", ")))
	}
	if len(m.MatchedBodyKeywords) > 0 {
		parts = append(parts, fmt.Sprintf("body keyword (%s)", strings.Join(m.MatchedBodyKeywords, ", ")))
	}

	if len(parts) == 0 {
		return "unknown"
	}

	return strings.Join(parts, " + ")
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// Send posts the matched email to the given webhook, pinging whoever is in ping
// timestampStyle is the letter for the <t:unix:X> tag, eg F for full date and time
func Send(webhookURL string, msg gmail.Message, match matcher.MatchResult, ping PingTarget, timestampStyle string, snippetLength int) error {
	mentionLine := buildMentionString(ping)

	discordTimestamp := fmt.Sprintf("<t:%d:%s>", msg.ReceivedUnix, timestampStyle)

	senderDisplay := msg.SenderEmail
	if msg.SenderName != "" {
		senderDisplay = fmt.Sprintf("%s (%s)", msg.SenderName, msg.SenderEmail)
	}

	e := embed{
		Title:       truncate(msg.Subject, 256),
		Description: truncate(msg.Body, snippetLength),
		Color:       0x5865F2, // discord blurple, no real reason just looks fine
		Fields: []embedField{
			{Name: "From", Value: senderDisplay, Inline: true},
			{Name: "Received", Value: discordTimestamp, Inline: true},
			{Name: "Matched On", Value: matchSummary(match), Inline: false},
		},
		Footer: embedFooter{Text: fmt.Sprintf("rule: %s", match.RuleName)},
	}

	if e.Title == "" {
		e.Title = "(no subject)"
	}
	if e.Description == "" {
		e.Description = "(empty body)"
	}

	payload := webhookPayload{
		Content: mentionLine,
		Embeds:  []embed{e},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("posting to webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord returned status %d", resp.StatusCode)
	}

	return nil
}
