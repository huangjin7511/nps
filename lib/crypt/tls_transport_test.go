package crypt

import (
	"encoding/json"
	"testing"
)

func TestPeerTransportDataRoundTrip(t *testing.T) {
	vkey := "test-vkey"
	certDER := []byte("test-cert-der")
	encoded := EncodePeerTransportData(vkey, certDER)
	if encoded == "" {
		t.Fatal("expected encoded transport data")
	}

	payload := struct {
		TransportData string `json:"transport_data"`
	}{
		TransportData: encoded,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	var decoded struct {
		TransportData string `json:"transport_data"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if decoded.TransportData != encoded {
		t.Fatalf("transport data changed after json round trip: got %q want %q", decoded.TransportData, encoded)
	}
	if !VerifyPeerTransportData(vkey, decoded.TransportData, certDER) {
		t.Fatal("expected encoded transport data to verify")
	}
}

func TestVerifyPeerTransportDataCompatibility(t *testing.T) {
	vkey := "test-vkey"
	certDER := []byte("test-cert-der")
	legacy := string(GetHMAC(vkey, certDER))

	if !VerifyPeerTransportData(vkey, legacy, certDER) {
		t.Fatal("expected legacy raw hmac payload to verify")
	}
	if VerifyPeerTransportData(vkey, "not-a-match", certDER) {
		t.Fatal("expected mismatched payload to fail verification")
	}
}
