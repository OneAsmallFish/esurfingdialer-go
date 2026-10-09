package netclient

import "testing"

func TestParsePortalConfigFuncfgAttributeOrderAndQuotes(t *testing.T) {
	raw := `<auth-url>https://auth.example</auth-url>
<ticket-url>https://ticket.example</ticket-url>
<funcfg>
  <QueryVerificateCodeStatus url="https://example/status" enable="1" />
  <QueryAuthCode enable='1' url='https://example/code' />
  <Disabled url="https://example/disabled" enable="0" />
</funcfg>`

	authURL, ticketURL, funcfg, err := parsePortalConfig(raw)
	if err != nil {
		t.Fatalf("parsePortalConfig: %v", err)
	}
	if authURL != "https://auth.example" || ticketURL != "https://ticket.example" {
		t.Fatalf("unexpected core URLs: auth=%q ticket=%q", authURL, ticketURL)
	}
	if got := funcfg["QueryVerificateCodeStatus"]; got != "https://example/status" {
		t.Fatalf("url-before-enable item missing: %q", got)
	}
	if got := funcfg["QueryAuthCode"]; got != "https://example/code" {
		t.Fatalf("single-quoted item missing: %q", got)
	}
	if _, ok := funcfg["Disabled"]; ok {
		t.Fatal("disabled item was collected")
	}
}
