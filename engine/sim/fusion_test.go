package sim

import "testing"

func TestFusionAndOvercharge(t *testing.T) {
	db := loadDB(t)
	w, _ := New(db, "fire", "fire", 3)
	w.Held = map[string]int{"band": 10, "lens": 10, "claw": 4}
	w.HeldOrder = []string{"band", "lens", "claw"}
	d0 := w.dmgM()
	var fo *Option
	for _, o := range w.options() {
		if o.Fuse {
			o := o
			fo = &o
		}
		if o.ID == "band" && o.Level != 10 {
			t.Errorf("band offered at level %d", o.Level)
		}
	}
	if fo == nil || fo.ID != "fz:razorband" {
		t.Fatalf("no Razor Band fusion offered: %+v", fo)
	}
	w.applyOpt(*fo)
	if w.heldCount() != 2 || w.Held["band"] != 10 || w.Held["lens"] != 10 || !w.fz("razorband") {
		t.Fatalf("after fusing: held %v order %v count %d", w.Held, w.HeldOrder, w.heldCount())
	}
	if w.dmgM() != d0 {
		t.Errorf("fusing changed base damage %v -> %v", d0, w.dmgM())
	}
	for _, o := range w.options() {
		if o.ID == "band" || o.ID == "lens" {
			t.Errorf("fused part %s still offered on its own", o.ID)
		}
	}
	w.applyOpt(Option{ID: "fz:razorband", Level: 10})
	if w.Held["band"] != 11 || w.pe("band") != 7.75 {
		t.Errorf("overcharging the fusion: band %d pe %v", w.Held["band"], w.pe("band"))
	}
	if OverTier(11) != 1 || OverTier(20) != 3 {
		t.Error("tiers")
	}
	if ItemInfo("fz:razorband") == nil || ItemInfo(FusedID("claw", "band")) == nil {
		t.Error("ItemInfo for fused ids")
	}
	w.dropHeld("fz:razorband")
	if w.Held["band"] != 0 || w.Held["lens"] != 0 {
		t.Error("dropping a fusion should drop its parts")
	}
}
