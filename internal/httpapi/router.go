package httpapi

import (
	"io/fs"
	"net/http"

	"task126-reinsurance/internal/alloc"
	"task126-reinsurance/internal/bordereaux"
	"task126-reinsurance/internal/contract"
	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/loss"
	"task126-reinsurance/internal/policy"
	"task126-reinsurance/internal/report"
	"task126-reinsurance/internal/store"
	"task126-reinsurance/internal/webfs"
)

// API wires all services behind a ServeMux. It owns the store and the service
// singletons; handlers close over it.
type API struct {
	store   *store.Store
	contracts *contract.Service
	policies  *policy.Service
	losses    *loss.Service
	alloc     *alloc.Engine
	bord      *bordereaux.Service
	report    *report.Service
	webFS     fs.FS
}

// New constructs an API bound to store s. The embedded web assets are served at
// "/" so the native frontend is reachable alongside the JSON routes.
func New(s *store.Store) (*API, error) {
	wfs, err := webfs.Sub()
	if err != nil {
		return nil, err
	}
	return &API{
		store:     s,
		contracts: contract.New(s),
		policies:  policy.New(s),
		losses:    loss.New(s),
		alloc:     alloc.New(s),
		bord:      bordereaux.New(s),
		report:    report.New(s),
		webFS:     wfs,
	}, nil
}

// Router builds the ServeMux with all routes.
func (a *API) Router() http.Handler {
	mux := http.NewServeMux()
	// Health.
	mux.HandleFunc("GET /healthz", a.healthz)
	// Contracts.
	mux.HandleFunc("POST /contracts", a.createContract)
	mux.HandleFunc("GET /contracts", a.listContracts)
	mux.HandleFunc("GET /contracts/{id}", a.getContract)
	mux.HandleFunc("PATCH /contracts/{id}", a.patchContract)
	mux.HandleFunc("POST /contracts/{id}/limits", a.contractLimits)
	mux.HandleFunc("POST /contracts/{id}/events/{tag}/allocate", a.allocateEvent)
	mux.HandleFunc("GET /contracts/{id}/events", a.listEvents)
	// Policies.
	mux.HandleFunc("POST /policies", a.createPolicy)
	mux.HandleFunc("GET /policies", a.listPolicies)
	mux.HandleFunc("GET /policies/{id}", a.getPolicy)
	// Losses.
	mux.HandleFunc("POST /losses", a.createLoss)
	mux.HandleFunc("GET /losses", a.listLosses)
	mux.HandleFunc("GET /losses/{id}", a.getLoss)
	mux.HandleFunc("POST /losses/{id}/allocate", a.allocateLoss)
	mux.HandleFunc("GET /losses/{id}/allocations", a.lossAllocations)
	// Bordereaux.
	mux.HandleFunc("POST /bordereaux", a.createBordereaux)
	mux.HandleFunc("GET /bordereaux", a.listBordereaux)
	mux.HandleFunc("GET /bordereaux/{id}", a.getBordereaux)
	mux.HandleFunc("POST /bordereaux/{id}/settle", a.settleBordereaux)
	mux.HandleFunc("POST /bordereaux/{id}/reopen", a.reopenBordereaux)
	// Reports.
	mux.HandleFunc("GET /reports/contracts/{id}", a.reportContract)
	mux.HandleFunc("GET /reports/bordereaux", a.reportBordereaux)
	mux.HandleFunc("GET /reports/losses", a.reportLosses)
	// Frontend: serve embedded files at root (falls through to index.html).
	mux.Handle("GET /", http.FileServer(http.FS(a.webFS)))
	return logging(cors(mux))
}

func (a *API) healthz(w http.ResponseWriter, r *http.Request) {
	if err := a.store.DB().PingContext(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "down", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) createContract(w http.ResponseWriter, r *http.Request) {
	var req contractCreateReq
	if err := decode(r, &req); err != nil { writeError(w, err); return }
	c, err := req.toDomain()
	if err != nil { writeError(w, err); return }
	if _, err := a.contracts.Create(r.Context(), c); err != nil { writeError(w, err); return }
	writeJSON(w, http.StatusCreated, contractToDTO(c))
}

func (a *API) listContracts(w http.ResponseWriter, r *http.Request) {
	contracts, err := a.contracts.List(r.Context())
	if err != nil { writeError(w, err); return }
	out := make([]contractDTO, 0, len(contracts))
	for _, c := range contracts { out = append(out, contractToDTO(c)) }
	writeJSON(w, http.StatusOK, out)
}

func (a *API) getContract(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(pathID(r, "id"))
	if err != nil { writeError(w, err); return }
	c, err := a.contracts.Get(r.Context(), id)
	if err != nil { writeError(w, err); return }
	writeJSON(w, http.StatusOK, contractToDTO(c))
}

func (a *API) patchContract(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(pathID(r, "id"))
	if err != nil { writeError(w, err); return }
	var req statusUpdateReq
	if err := decode(r, &req); err != nil { writeError(w, err); return }
	c, err := a.contracts.TransitionStatus(r.Context(), id, domain.ContractStatus(req.Status))
	if err != nil { writeError(w, err); return }
	writeJSON(w, http.StatusOK, contractToDTO(c))
}

func (a *API) contractLimits(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(pathID(r, "id"))
	if err != nil { writeError(w, err); return }
	cl, err := a.alloc.LimitsFor(r.Context(), id)
	if err != nil { writeError(w, err); return }
	if cl == nil {
		writeJSON(w, http.StatusOK, map[string]any{"limit_remaining_cents": 0, "reinstatements_used": 0})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"limit_remaining_cents": moneyToCents(cl.LimitRemaining),
		"reinstatements_used":   cl.ReinstatementsUsed,
		"updated_at":           cl.UpdatedAt,
	})
}

func (a *API) allocateEvent(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(pathID(r, "id"))
	if err != nil { writeError(w, err); return }
	tag := pathID(r, "tag")
	allocs, err := a.alloc.AllocateEvent(r.Context(), id, tag)
	if err != nil { writeError(w, err); return }
	out := make([]allocationDTO, 0, len(allocs))
	for _, al := range allocs { out = append(out, allocationToDTO(al)) }
	writeJSON(w, http.StatusOK, out)
}

func (a *API) listEvents(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(pathID(r, "id"))
	if err != nil { writeError(w, err); return }
	events, err := a.alloc.ContractEvents(r.Context(), id)
	if err != nil { writeError(w, err); return }
	out := make([]map[string]any, 0, len(events))
	for _, e := range events {
		out = append(out, map[string]any{
			"id": e.ID, "event_tag": e.EventTag,
			"accumulated_loss_cents": moneyToCents(e.AccumulatedLoss), "allocated": e.Allocated,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) createPolicy(w http.ResponseWriter, r *http.Request) {
	var req policyCreateReq
	if err := decode(r, &req); err != nil { writeError(w, err); return }
	p, err := req.toDomain()
	if err != nil { writeError(w, err); return }
	if _, err := a.policies.Create(r.Context(), p); err != nil { writeError(w, err); return }
	writeJSON(w, http.StatusCreated, policyToDTO(p))
}

func (a *API) listPolicies(w http.ResponseWriter, r *http.Request) {
	policies, err := a.policies.List(r.Context())
	if err != nil { writeError(w, err); return }
	out := make([]policyDTO, 0, len(policies))
	for _, p := range policies { out = append(out, policyToDTO(p)) }
	writeJSON(w, http.StatusOK, out)
}

func (a *API) getPolicy(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(pathID(r, "id"))
	if err != nil { writeError(w, err); return }
	p, err := a.policies.Get(r.Context(), id)
	if err != nil { writeError(w, err); return }
	writeJSON(w, http.StatusOK, policyToDTO(p))
}

func (a *API) createLoss(w http.ResponseWriter, r *http.Request) {
	var req lossCreateReq
	if err := decode(r, &req); err != nil { writeError(w, err); return }
	l, err := req.toDomain()
	if err != nil { writeError(w, err); return }
	if _, err := a.losses.Create(r.Context(), l); err != nil { writeError(w, err); return }
	writeJSON(w, http.StatusCreated, lossToDTO(l))
}

func (a *API) listLosses(w http.ResponseWriter, r *http.Request) {
	losses, err := a.losses.List(r.Context())
	if err != nil { writeError(w, err); return }
	out := make([]lossDTO, 0, len(losses))
	for _, l := range losses { out = append(out, lossToDTO(l)) }
	writeJSON(w, http.StatusOK, out)
}

func (a *API) getLoss(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(pathID(r, "id"))
	if err != nil { writeError(w, err); return }
	l, err := a.losses.Get(r.Context(), id)
	if err != nil { writeError(w, err); return }
	writeJSON(w, http.StatusOK, lossToDTO(l))
}

func (a *API) allocateLoss(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(pathID(r, "id"))
	if err != nil { writeError(w, err); return }
	allocs, err := a.alloc.AllocateLoss(r.Context(), id)
	if err != nil { writeError(w, err); return }
	out := make([]allocationDTO, 0, len(allocs))
	for _, al := range allocs { out = append(out, allocationToDTO(al)) }
	writeJSON(w, http.StatusOK, out)
}

func (a *API) lossAllocations(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(pathID(r, "id"))
	if err != nil { writeError(w, err); return }
	allocs, err := a.alloc.AllocationsForLoss(r.Context(), id)
	if err != nil { writeError(w, err); return }
	out := make([]allocationDTO, 0, len(allocs))
	for _, al := range allocs { out = append(out, allocationToDTO(al)) }
	writeJSON(w, http.StatusOK, out)
}

func (a *API) createBordereaux(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ContractID    int64 `json:"contract_id"`
		PeriodYear    int   `json:"period_year"`
		PeriodQuarter int   `json:"period_quarter"`
	}
	if err := decode(r, &req); err != nil { writeError(w, err); return }
	b, err := a.bord.Ensure(r.Context(), req.ContractID, req.PeriodYear, req.PeriodQuarter)
	if err != nil { writeError(w, err); return }
	writeJSON(w, http.StatusOK, bordereauxToDTO(b))
}

func (a *API) listBordereaux(w http.ResponseWriter, r *http.Request) {
	bords, err := a.bord.List(r.Context())
	if err != nil { writeError(w, err); return }
	out := make([]bordereauxDTO, 0, len(bords))
	for _, b := range bords { out = append(out, bordereauxToDTO(b)) }
	writeJSON(w, http.StatusOK, out)
}

func (a *API) getBordereaux(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(pathID(r, "id"))
	if err != nil { writeError(w, err); return }
	b, err := a.bord.Get(r.Context(), id)
	if err != nil { writeError(w, err); return }
	writeJSON(w, http.StatusOK, bordereauxToDTO(b))
}

func (a *API) settleBordereaux(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(pathID(r, "id"))
	if err != nil { writeError(w, err); return }
	b, err := a.bord.Settle(r.Context(), id)
	if err != nil { writeError(w, err); return }
	writeJSON(w, http.StatusOK, bordereauxToDTO(b))
}

func (a *API) reopenBordereaux(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(pathID(r, "id"))
	if err != nil { writeError(w, err); return }
	b, err := a.bord.Reopen(r.Context(), id)
	if err != nil { writeError(w, err); return }
	writeJSON(w, http.StatusOK, bordereauxToDTO(b))
}

func (a *API) reportContract(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathID(pathID(r, "id"))
	if err != nil { writeError(w, err); return }
	s, err := a.report.Contract(r.Context(), id)
	if err != nil { writeError(w, err); return }
	writeJSON(w, http.StatusOK, map[string]any{
		"contract":             contractToDTO(s.Contract),
		"policy_count":         s.PolicyCount,
		"loss_count":           s.LossCount,
		"total_recovered_cents": moneyToCents(s.TotalRecovered),
		"reinstatements_used":  s.ReinstatementsUsed,
		"limit_remaining_cents": moneyToCents(s.LimitRemaining),
		"bordereaux_count":     s.BordereauxCount,
		"net_balance_cents":    moneyToCents(s.NetBalance),
	})
}

func (a *API) reportBordereaux(w http.ResponseWriter, r *http.Request) {
	bords, err := a.report.BordereauxAll(r.Context())
	if err != nil { writeError(w, err); return }
	out := make([]bordereauxDTO, 0, len(bords))
	for _, b := range bords { out = append(out, bordereauxToDTO(b)) }
	writeJSON(w, http.StatusOK, out)
}

func (a *API) reportLosses(w http.ResponseWriter, r *http.Request) {
	reps, err := a.report.LossesAll(r.Context())
	if err != nil { writeError(w, err); return }
	out := make([]map[string]any, 0, len(reps))
	for _, lr := range reps {
		allocs := make([]allocationDTO, 0, len(lr.Allocations))
		for _, al := range lr.Allocations { allocs = append(allocs, allocationToDTO(al)) }
		out = append(out, map[string]any{
			"loss":               lossToDTO(lr.Loss),
			"allocations":        allocs,
			"net_retained_cents": moneyToCents(lr.NetRetained),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// logging records each request line.
func logging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
	})
}

// cors adds permissive headers for the embedded frontend origin.
func cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}
