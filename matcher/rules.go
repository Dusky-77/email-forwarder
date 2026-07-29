package matcher

import (
	"strings"

	"email-forwarder/gmail"
)

// MatchResult tells the caller which rule fired and which specific things matched
// mostly so discord/forwarder.go can show "matched: subject keyword" etc in the embed
type MatchResult struct {
	RuleName string

	MatchedSenderEmail  bool
	MatchedSenderDomain bool
	MatchedSenderName   bool

	MatchedSubjectKeywords []string
	MatchedBodyKeywords    []string
}

func (r MatchResult) matchedAnything() bool {
	return r.MatchedSenderEmail || r.MatchedSenderDomain || r.MatchedSenderName ||
		len(r.MatchedSubjectKeywords) > 0 || len(r.MatchedBodyKeywords) > 0
}

// RuleLike is the subset of config.Rule that the matcher needs
// defined here instead of importing the config package directly to avoid an import cycle
// since config.go lives in package main and main imports matcher
type RuleLike struct {
	Name                string
	SenderEmails        []string
	SenderDomains       []string
	SenderNameContains  []string
	Keywords            []string
}

// Find runs the message against every rule in order and returns the first match
// returns nil if nothing matched
func Find(msg gmail.Message, rules []RuleLike) *MatchResult {
	for _, rule := range rules {
		result := checkRule(msg, rule)
		if result.matchedAnything() {
			return &result
		}
	}
	return nil
}

func checkRule(msg gmail.Message, rule RuleLike) MatchResult {
	result := MatchResult{RuleName: rule.Name}

	// if a field has entries, ALL configured fields must pass (AND between fields)
	// but we still record which specific ones matched for the embed later

	senderEmailLower := strings.ToLower(msg.SenderEmail)
	senderNameLower := strings.ToLower(msg.SenderName)

	hasSenderEmailRule := len(rule.SenderEmails) > 0
	hasSenderDomainRule := len(rule.SenderDomains) > 0
	hasSenderNameRule := len(rule.SenderNameContains) > 0
	hasKeywordRule := len(rule.Keywords) > 0

	if hasSenderEmailRule {
		for _, e := range rule.SenderEmails {
			if strings.EqualFold(e, msg.SenderEmail) {
				result.MatchedSenderEmail = true
				break
			}
		}
		if !result.MatchedSenderEmail {
			return MatchResult{RuleName: rule.Name} // required field failed, whole rule fails
		}
	}

	if hasSenderDomainRule {
		for _, d := range rule.SenderDomains {
			if strings.HasSuffix(senderEmailLower, "@"+strings.ToLower(d)) {
				result.MatchedSenderDomain = true
				break
			}
		}
		if !result.MatchedSenderDomain {
			return MatchResult{RuleName: rule.Name}
		}
	}

	if hasSenderNameRule {
		for _, n := range rule.SenderNameContains {
			if strings.Contains(senderNameLower, strings.ToLower(n)) {
				result.MatchedSenderName = true
				break
			}
		}
		if !result.MatchedSenderName {
			return MatchResult{RuleName: rule.Name}
		}
	}

	if hasKeywordRule {
		for _, kw := range rule.Keywords {
			// exact case sensitive, no ToLower here on purpose
			if strings.Contains(msg.Subject, kw) {
				result.MatchedSubjectKeywords = append(result.MatchedSubjectKeywords, kw)
			}
			if strings.Contains(msg.Body, kw) {
				result.MatchedBodyKeywords = append(result.MatchedBodyKeywords, kw)
			}
		}
		if len(result.MatchedSubjectKeywords) == 0 && len(result.MatchedBodyKeywords) == 0 {
			return MatchResult{RuleName: rule.Name}
		}
	}

	// if the rule had zero fields configured at all, it cant match anything, skip it
	if !hasSenderEmailRule && !hasSenderDomainRule && !hasSenderNameRule && !hasKeywordRule {
		return MatchResult{RuleName: rule.Name}
	}

	return result
}
