package handlers

import (
	"strings"
	"testing"

	"github.com/unknovs/auth-test-harness/env"
	"github.com/unknovs/auth-test-harness/utils"
)

// The two test people, assembled at run time rather than written as literals: an
// identifier-shaped constant in a published repository cannot be told from a real
// person's code. Each person is one repeated digit.
var (
	personA = testIDCodeLV("1")
	personB = testIDCodeLV("2")
)

func testIDCodeLV(digit string) string {
	return "PNOLV-" + strings.Repeat(digit, 6) + "-" + strings.Repeat(digit, 5)
}

func testHandler() *OAuthHandler {
	key, err := utils.NewSigningKey()
	if err != nil {
		panic(err)
	}

	return NewOAuthHandler(&env.Config{
		Protocol:            "http",
		Host:                "idp.test:8080",
		BasicAuthValue:      "dGVzdDp0ZXN0",
		TokenExpirationMin:  10,
		ScopesSupported:     []string{"openid"},
		ACRValuesSupported:  []string{flowMobileID, flowSCPlugin, flowEIDScan, flowDirectory, flowDirectoryGuest},
		SerialNumber:        personA,
		MobileGivenName:     "Jane",
		MobileFamilyName:    "Mobile",
		MobileSerialNumber:  personA,
		SCGivenName:         "John",
		SCFamilyName:        "Cardreader",
		SCSerialNumber:      personA,
		EIDScanGivenName:    "Erik",
		EIDScanFamilyName:   "Scanner",
		EIDScanSerialNumber: personB,
		DirectoryGivenName:  "Ilze",
		DirectoryFamilyName: "Ozola",
	}, utils.NewInMemoryStore(), key)
}

// Each requested flow must come back with its own name profile and with the
// method it asked for reported in the amr, so a caller can verify that the
// method it forced is the method it got.
func TestGenerateUserInfoPerFlow(t *testing.T) {
	h := testHandler()

	cases := []struct {
		name       string
		acrValues  string
		wantAMR    string
		wantName   string
		wantGiven  string
		wantFamily string
		wantSerial string
	}{
		{
			name:       "mobile",
			acrValues:  flowMobileID,
			wantAMR:    amrMethodPrefix + "mobileid",
			wantGiven:  "Jane",
			wantFamily: "Mobile",
			wantName:   "Jane Mobile",
			wantSerial: personA,
		},
		{
			name:       "smart card",
			acrValues:  flowSCPlugin,
			wantAMR:    amrMethodPrefix + "sc_plugin",
			wantGiven:  "John",
			wantFamily: "Cardreader",
			wantName:   "John Cardreader",
			wantSerial: personA,
		},
		{
			// Its own identity code, so this profile stands in for a second
			// party in a flow where a document is shared between two people.
			name:       "eID Scan",
			acrValues:  flowEIDScan,
			wantAMR:    amrMethodPrefix + "mobile-eid",
			wantGiven:  "Erik",
			wantFamily: "Scanner",
			wantName:   "Erik Scanner",
			wantSerial: personB,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := h.generateUserInfo(tc.acrValues)

			if len(got.AMR) != 1 || got.AMR[0] != tc.wantAMR {
				t.Errorf("amr = %v, want [%s]", got.AMR, tc.wantAMR)
			}
			if got.GivenName != tc.wantGiven || got.FamilyName != tc.wantFamily {
				t.Errorf("name parts = %q %q, want %q %q", got.GivenName, got.FamilyName, tc.wantGiven, tc.wantFamily)
			}
			if got.Name != tc.wantName {
				t.Errorf("name = %q, want %q", got.Name, tc.wantName)
			}
			if got.SerialNumber != tc.wantSerial {
				t.Errorf("serial_number = %q, want %q", got.SerialNumber, tc.wantSerial)
			}
		})
	}
}

// A flow the service was not configured for still reports a well-formed method
// URN (its own trailing segment), never an invented one.
func TestAMRForUnknownFlowKeepsItsSegment(t *testing.T) {
	if got, want := amrForFlow("urn:example:authentication:flow:passkey"), amrMethodPrefix+"passkey"; got != want {
		t.Errorf("amr = %q, want %q", got, want)
	}
}

// An empty or segment-less request falls back to the smart-card method rather
// than emitting a bare prefix.
func TestAMRForEmptyFlowFallsBackToSmartCard(t *testing.T) {
	for _, in := range []string{"", "nosegments"} {
		if got, want := amrForFlow(in), amrMethodPrefix+"sc_plugin"; got != want {
			t.Errorf("amrForFlow(%q) = %q, want %q", in, got, want)
		}
	}
}
