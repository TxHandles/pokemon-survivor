package sim

import "testing"

func TestTrader(t *testing.T) {
	db := loadDB(t)
	w, _ := New(db, "fire", "fire", 4)
	w.E = nil
	w.TestTrader()
	if w.Trader == nil || len(w.Trader.Stock) != 3 {
		t.Fatal("no trader")
	}
	w.Trader.X, w.Trader.Y = w.P.X, w.P.Y
	for i := 0; i < 80 && w.PendingKind(0) != "shop"; i++ {
		w.Step(1.0/60, Input{})
	}
	if w.PendingKind(0) != "shop" || !w.Waiting() {
		t.Fatalf("shop didn't open: state %v", w.State)
	}
	c0 := w.Coins
	id := w.Trader.Stock[0].ID
	w.ShopFor(0, 0)
	if !w.rare(id) || w.Coins != c0-float64(w.Trader.Stock[0].Price) || !w.Trader.Stock[0].Sold {
		t.Fatalf("buy failed: rare %v coins %v -> %v", w.rare(id), c0, w.Coins)
	}
	w.ShopFor(0, 0) // already sold: nothing
	if w.PendingKind(0) != "shop" {
		t.Fatal("shop should stay open after a purchase")
	}
	w.ShopFor(0, -1)
	if w.Waiting() {
		t.Fatal("leaving should resume play")
	}
	for i := 0; i < 120; i++ { // standing there doesn't reopen it
		w.Step(1.0/60, Input{})
	}
	if w.PendingKind(0) == "shop" {
		t.Fatal("reopened without stepping away")
	}
}
