package handlers

import (
	"testing"

	"github.com/unknovs/auth-test-harness/env"
	"github.com/unknovs/auth-test-harness/utils"
)

func testHandler() *OAuthHandler {
	return NewOAuthHandler(&env.Config{
		SerialNumber:        "PNOLV-123456-12345",
		MobileGivenName:     "Jane",
		MobileFamilyName:    "Mobile",
		MobileSerialNumber:  "PNOLV-123456-12345",
		SCGivenName:         "John",
		SCFamilyName:        "Cardreader",
		SCSerialNumber:      "PNOLV-123456-12345",
		EIDScanGivenName:    "Erik",
		EIDScanFamilyName:   "Scanner",
		EIDScanSerialNumber: "PNOLV-654321-54321",
	}, utils.NewInMemoryStore())
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
			wantSerial: "PNOLV-123456-12345",
		},
		{
			name:       "smart card",
			acrValues:  flowSCPlugin,
			wantAMR:    amrMethodPrefix + "sc_plugin",
			wantGiven:  "John",
			wantFamily: "Cardreader",
			wantName:   "John Cardreader",
			wantSerial: "PNOLV-123456-12345",
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
			wantSerial: "PNOLV-654321-54321",
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
