package main

import (
	"context"
	"log"
	"time"

	"email-forwarder/discord"
	"email-forwarder/gmail"
	"email-forwarder/matcher"
	"email-forwarder/store"

	gmailapi "google.golang.org/api/gmail/v1"
)

func main() {
	log.Println("starting email forwarder")

	seenStore, err := store.New(StoreDir)
	if err != nil {
		log.Fatalf("could not init store: %v", err)
	}

	if len(Accounts) == 0 {
		log.Fatal("no accounts configured, add some in config.go")
	}

	// one goroutine per account so a slow/stuck account doesnt block the others
	// authResult reports true if that account made it past auth and is actually polling
	authResult := make(chan bool, len(Accounts))

	for _, acc := range Accounts {
		go runAccountLoop(acc, seenStore, authResult)
	}

	// wait for every account to report in on whether it started polling
	// if none of them made it, theres nothing left running, exit instead of hanging forever
	liveAccounts := 0
	for i := 0; i < len(Accounts); i++ {
		if <-authResult {
			liveAccounts++
		}
	}

	if liveAccounts == 0 {
		log.Fatal("no accounts authenticated successfully, nothing to poll, exiting")
	}

	log.Printf("%d/%d account(s) polling, running", liveAccounts, len(Accounts))

	select {} // block forever, ctrl+c to stop
}

func runAccountLoop(acc GmailAccount, seenStore *store.Store, authResult chan<- bool) {
	ctx := context.Background()

	svc, err := gmail.GetClient(ctx, acc.CredentialsFile, acc.TokenFile)
	if err != nil {
		log.Printf("[%s] failed to authenticate: %v", acc.Name, err)
		authResult <- false
		return
	}

	authResult <- true

	log.Printf("[%s] authenticated, polling every %d minutes", acc.Name, PollIntervalMinutes)

	ticker := time.NewTicker(time.Duration(PollIntervalMinutes) * time.Minute)
	defer ticker.Stop()

	// run once immediately on startup, then wait for the ticker after that
	pollOnce(acc, svc, seenStore)

	for range ticker.C {
		pollOnce(acc, svc, seenStore)
	}
}

func pollOnce(acc GmailAccount, svc *gmailapi.Service, seenStore *store.Store) {
	lastHistoryID := seenStore.GetLastHistoryID(acc.Name)

	messages, newHistoryID, err := gmail.FetchNew(svc, lastHistoryID)
	if err != nil {
		log.Printf("[%s] fetch failed: %v", acc.Name, err)
		return
	}

	if len(messages) == 0 {
		// still bump the stored historyId even with no new mail, keeps us moving forward
		if newHistoryID > 0 {
			seenStore.SetLastHistoryID(acc.Name, newHistoryID)
		}
		return
	}

	log.Printf("[%s] found %d new message(s)", acc.Name, len(messages))

	convertedRules := toRuleLike(Rules)

	for _, msg := range messages {
		match := matcher.Find(msg, convertedRules)
		if match == nil {
			continue
		}

		rule := findRuleByName(match.RuleName)
		if rule == nil {
			continue
		}

		ping := rule.Ping
		if len(ping.UserIDs) == 0 && len(ping.RoleIDs) == 0 {
			ping = DefaultPing
		}

		err := discord.Send(
			rule.WebhookURL,
			msg,
			*match,
			discord.PingTarget{UserIDs: ping.UserIDs, RoleIDs: ping.RoleIDs},
			DiscordTimestampStyle,
			BodySnippetLength,
		)
		if err != nil {
			log.Printf("[%s] failed to forward message %s: %v", acc.Name, msg.ID, err)
			continue
		}

		log.Printf("[%s] forwarded message %s (rule: %s)", acc.Name, msg.ID, match.RuleName)
	}

	if err := seenStore.SetLastHistoryID(acc.Name, newHistoryID); err != nil {
		log.Printf("[%s] failed to save history id: %v", acc.Name, err)
	}
}

// toRuleLike converts our config.Rule slice into matcher.RuleLike
// keeps the matcher package decoupled from config/main
func toRuleLike(rules []Rule) []matcher.RuleLike {
	result := make([]matcher.RuleLike, len(rules))
	for i, r := range rules {
		result[i] = matcher.RuleLike{
			Name:               r.Name,
			SenderEmails:       r.SenderEmails,
			SenderDomains:      r.SenderDomains,
			SenderNameContains: r.SenderNameContains,
			Keywords:           r.Keywords,
		}
	}
	return result
}

func findRuleByName(name string) *Rule {
	for i := range Rules {
		if Rules[i].Name == name {
			return &Rules[i]
		}
	}
	return nil
}
