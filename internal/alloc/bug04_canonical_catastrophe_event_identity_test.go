package alloc

import (
	"context"
	"testing"
	"time"

	"task126-reinsurance/internal/contract"
	"task126-reinsurance/internal/domain"
	"task126-reinsurance/internal/loss"
	"task126-reinsurance/internal/policy"
	"task126-reinsurance/internal/store"
)

func TestBug04_CatastropheEventUsesCanonicalTagIdentity(t *testing.T) {
	ctx := context.Background(); s, err := store.Open(ctx, ":memory:"); if err != nil { t.Fatal(err) }; t.Cleanup(func(){ _ = s.Close() })
	c, err := contract.New(s).Create(ctx, &domain.Contract{Code:"CAT-CANON", Type:domain.CatXL, AttachmentPoint:500000, Limit:2000000, StartDate:time.Date(2026,1,1,0,0,0,0,time.UTC), EndDate:time.Date(2026,12,31,0,0,0,0,time.UTC)})
	if err != nil { t.Fatal(err) }
	ps, ls := policy.New(s), loss.New(s)
	var ids []int64
	for _, tc := range []struct{ no, tag string; paid domain.Money }{{"CANON-1", " Storm-9 ", 500000}, {"CANON-2", "storm-9", 1500000}} {
		p,err:=ps.Create(ctx,&domain.Policy{PolicyNo:tc.no+"-P",ContractID:c.ID,SumInsured:5000000,OriginalPremium:100000,StartDate:time.Date(2026,1,1,0,0,0,0,time.UTC),EndDate:time.Date(2026,12,31,0,0,0,0,time.UTC)}); if err != nil { t.Fatal(err) }
		l,err:=ls.Create(ctx,&domain.Loss{ClaimNo:tc.no,PolicyID:p.ID,PaidAmount:tc.paid,EventTag:tc.tag,OccurrenceDate:time.Date(2026,8,1,0,0,0,0,time.UTC)}); if err != nil { t.Fatal(err) }; ids=append(ids,l.ID)
	}
	got,err:=New(s).AllocateEvent(ctx,c.ID,"STORM-9"); if err != nil { t.Fatal(err) }
	if len(got)!=1 || got[0].RecoveredAmount!=1500000 { t.Fatalf("equivalent tags must produce one combined event recovery, got %+v",got) }
	for _,id:=range ids { l,err:=ls.Get(ctx,id); if err!=nil { t.Fatal(err) }; if l.Status!=domain.LossAllocated { t.Fatalf("loss %d was not sealed with its canonical event",id) } }
}
