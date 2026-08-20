package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestFrontendServedAndBusinessFlow verifies the embedded frontend is served
// and a real business write/read flow works over HTTP.
func TestFrontendServedAndBusinessFlow(t *testing.T) {
	api, _ := openAPI(t)
	h := api.Router()
	// 1. Frontend page is served and non-empty.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("frontend status: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "再保险") {
		t.Fatalf("frontend body missing title: %q", w.Body.String()[:min(120, w.Body.Len())])
	}
	// 2. Create a contract, policy, loss, allocate, settle bordereaux -- end-to-end.
	contractBody := `{"code":"FE-1","type":"quota_share","cession_rate":0.5,"ceding_commission_rate":0.1,"broker_rate":0.02,"start_date":"2026-01-01","end_date":"2026-12-31"}`
	var c contractDTO
	postJSON(t, h, "/contracts", contractBody, &c)
	policyBody := `{"policy_no":"PFE-1","contract_id":CONTRACT,"sum_insured_cents":100000000,"original_premium_cents":400000,"start_date":"2026-07-01","end_date":"2026-09-30"}`
	var p policyDTO
	postJSON(t, h, "/policies", strings.ReplaceAll(policyBody, "CONTRACT", itoa(c.ID)), &p)
	lossBody := `{"claim_no":"LFE-1","policy_id":POLICY,"occurrence_date":"2026-08-15","paid_amount_cents":1000000}`
	var l lossDTO
	postJSON(t, h, "/losses", strings.ReplaceAll(lossBody, "POLICY", itoa(p.ID)), &l)
	var allocs []allocationDTO
	postJSON(t, h, "/losses/"+itoa(l.ID)+"/allocate", "{}", &allocs)
	if len(allocs) != 1 || allocs[0].RecoveredCents != 500000 {
		t.Fatalf("allocate: %+v", allocs)
	}
	bordBody := `{"contract_id":CONTRACT,"period_year":2026,"period_quarter":3}`
	var b bordereauxDTO
	postJSON(t, h, "/bordereaux", strings.ReplaceAll(bordBody, "CONTRACT", itoa(c.ID)), &b)
	var settled bordereauxDTO
	postJSON(t, h, "/bordereaux/"+itoa(b.ID)+"/settle", "{}", &settled)
	if settled.Status != "settled" {
		t.Fatalf("settle status: %s", settled.Status)
	}
	if settled.CededPremiumCents != 200000 || settled.RecoveredLossCents != 500000 {
		t.Fatalf("bord aggregates: %+v", settled)
	}
	// 3. Contract summary report.
	var rep map[string]any
	getJSON(t, h, "/reports/contracts/"+itoa(c.ID), &rep)
	if rep["loss_count"].(float64) != 1 {
		t.Fatalf("report loss count: %v", rep["loss_count"])
	}
}

func postJSON(t *testing.T, h http.Handler, path, body string, dst any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code >= 400 {
		t.Fatalf("POST %s -> %d: %s", path, w.Code, w.Body.String())
	}
	if dst != nil {
		dec := jsonNewDecoder(w.Body)
		if err := dec.Decode(dst); err != nil {
			t.Fatalf("decode %s: %v body=%s", path, err, w.Body.String())
		}
	}
}

func getJSON(t *testing.T, h http.Handler, path string, dst any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code >= 400 {
		t.Fatalf("GET %s -> %d: %s", path, w.Code, w.Body.String())
	}
	dec := jsonNewDecoder(w.Body)
	if err := dec.Decode(dst); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func itoa(n int64) string {
	return intToStr(n)
}
