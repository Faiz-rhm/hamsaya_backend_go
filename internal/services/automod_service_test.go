package services

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRuleFields(t *testing.T) {
	cases := []struct {
		name     string
		pattern  string
		action   string
		severity string
		isRegex  bool
		wantErr  string
	}{
		{"empty pattern", "", "flag", "medium", false, "pattern required"},
		{"whitespace pattern", "   ", "flag", "medium", false, "pattern required"},
		{"invalid action", "spam", "delete", "medium", false, "invalid action"},
		{"invalid severity", "spam", "flag", "extreme", false, "invalid severity"},
		{"invalid regex", "[unclosed", "flag", "medium", true, "invalid regex"},
		{"valid substring", "spam", "flag", "medium", false, ""},
		{"valid regex", `^(buy|now)\b`, "block", "high", true, ""},
		{"valid block+critical", "fraud", "block", "critical", false, ""},
		{"valid shadow alias", "lurk", "shadow", "low", false, ""},
		{"valid all severities low", "x", "flag", "low", false, ""},
		{"valid all severities critical", "y", "flag", "critical", false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRuleFields(tc.pattern, tc.action, tc.severity, tc.isRegex)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func mustCompile(t *testing.T, r AutomodRule) compiledRule {
	t.Helper()
	c, err := compileRule(r)
	require.NoError(t, err)
	return c
}

func ruleWithID(action, severity, pattern string, isRegex bool, desc string) AutomodRule {
	var d *string
	if desc != "" {
		d = &desc
	}
	return AutomodRule{
		ID:          uuid.New(),
		Pattern:     pattern,
		IsRegex:     isRegex,
		Action:      action,
		Severity:    severity,
		Enabled:     true,
		Description: d,
	}
}

func TestScanCompiledRules_NoRules(t *testing.T) {
	got := scanCompiledRules(nil, "anything")
	assert.Equal(t, AutomodMatch{}, got)
}

func TestScanCompiledRules_NoMatch(t *testing.T) {
	rules := []compiledRule{
		mustCompile(t, ruleWithID("flag", "medium", "spam", false, "")),
	}
	got := scanCompiledRules(rules, "ordinary content")
	assert.Equal(t, uuid.Nil, got.RuleID)
}

func TestScanCompiledRules_SubstringCaseInsensitive(t *testing.T) {
	rule := ruleWithID("flag", "medium", "Casino", false, "no gambling")
	rules := []compiledRule{mustCompile(t, rule)}
	got := scanCompiledRules(rules, "Welcome to my CASINO night")
	assert.Equal(t, rule.ID, got.RuleID)
	assert.Equal(t, "flag", got.Action)
	assert.Equal(t, "no gambling", got.Description)
}

func TestScanCompiledRules_RegexCaseInsensitive(t *testing.T) {
	rule := ruleWithID("block", "high", `\bbuy\s+now\b`, true, "promo phrase")
	rules := []compiledRule{mustCompile(t, rule)}
	got := scanCompiledRules(rules, "Limited time — BUY  NOW or miss it")
	assert.Equal(t, rule.ID, got.RuleID)
	assert.Equal(t, "block", got.Action)
}

func TestScanCompiledRules_BlockBeatsFlag(t *testing.T) {
	flagRule := ruleWithID("flag", "critical", "warn", false, "flag desc")
	blockRule := ruleWithID("block", "low", "block", false, "block desc")
	rules := []compiledRule{
		mustCompile(t, flagRule),
		mustCompile(t, blockRule),
	}
	got := scanCompiledRules(rules, "this triggers warn AND block in one pass")
	// block must win even though flag is critical and block is only low.
	assert.Equal(t, blockRule.ID, got.RuleID)
	assert.Equal(t, "block", got.Action)
}

func TestScanCompiledRules_HigherSeverityWinsWithinSameAction(t *testing.T) {
	low := ruleWithID("flag", "low", "alpha", false, "")
	medium := ruleWithID("flag", "medium", "alpha", false, "")
	high := ruleWithID("flag", "high", "alpha", false, "")
	rules := []compiledRule{
		mustCompile(t, low),
		mustCompile(t, medium),
		mustCompile(t, high),
	}
	got := scanCompiledRules(rules, "alpha bravo")
	assert.Equal(t, high.ID, got.RuleID)
	assert.Equal(t, "high", got.Severity)
}

func TestScanCompiledRules_FirstStrictestWinsOnTie(t *testing.T) {
	a := ruleWithID("block", "high", "bad", false, "first")
	b := ruleWithID("block", "high", "bad", false, "second")
	rules := []compiledRule{mustCompile(t, a), mustCompile(t, b)}
	got := scanCompiledRules(rules, "very bad words")
	// Implementation uses '>' not '>=' so the first match wins on tie.
	assert.Equal(t, a.ID, got.RuleID)
	assert.Equal(t, "first", got.Description)
}

func TestScanCompiledRules_DescriptionFallback(t *testing.T) {
	rule := ruleWithID("flag", "medium", "term", false, "")
	rules := []compiledRule{mustCompile(t, rule)}
	got := scanCompiledRules(rules, "term term")
	assert.Equal(t, rule.ID, got.RuleID)
	// matchUserMessage should produce the fallback when description is empty.
	assert.Equal(t, "matched a content rule", matchUserMessage(got))
}

func TestMatchUserMessage(t *testing.T) {
	assert.Equal(t, "use this", matchUserMessage(AutomodMatch{Description: "use this"}))
	assert.Equal(t, "matched a content rule", matchUserMessage(AutomodMatch{}))
}

func TestCompileRule_InvalidRegexReturnsError(t *testing.T) {
	_, err := compileRule(ruleWithID("flag", "low", "[unclosed", true, ""))
	require.Error(t, err)
}

func TestCompileRule_LiteralLowercased(t *testing.T) {
	c, err := compileRule(ruleWithID("flag", "low", "MixEdCaSe", false, ""))
	require.NoError(t, err)
	assert.Equal(t, "mixedcase", c.literal)
	assert.Nil(t, c.regex)
	assert.True(t, strings.HasPrefix(c.literal, "mix"))
}
