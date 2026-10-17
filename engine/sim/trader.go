package sim

import "math"

// The travelling trader: every 2:30 there is a small chance one sets up near the party for a while.
// Standing next to them opens their shop: rare held items (never offered on level-ups) bought with
// the run's coins. Rare items don't take a held-item slot.

const (
	TraderEvery = 150.0 // seconds between rolls
	TraderP     = .01   // chance per roll
	TraderStay  = 90.0  // seconds before they pack up
	traderOpenT = .8    // seconds standing next to them to open the shop
	traderR     = 55.0
)

// RareItem is a trader-only held item.
type RareItem struct {
	ID, Name, Desc string
	Price          float64 // base price in run coins (grows as the run goes on)
}

var RareItems = []RareItem{
	{"souldew", "Soul Dew", "+20% damage", 70},
	{"vest", "Assault Vest", "-20% damage taken", 60},
	{"eviolite", "Eviolite", "+25% max HP (heals the difference)", 45},
	{"powder", "Quick Powder", "-12% cooldowns and +10% move speed", 55},
	{"luckyegg", "Lucky Egg", "+30% XP", 35},
	{"amulet", "Amulet Coin", "+50% coins", 30},
	{"shellbell", "Shell Bell", "Heal 2% of the damage you deal", 50},
	{"kingsrock", "King's Rock", "Hits have a 10% chance to make foes flinch (stunned 0.5s)", 50},
	{"zoomlens", "Zoom Lens", "+25% damage to bosses and elites", 45},
	{"policy", "Weakness Policy", "Getting hit gives +35% damage for 5 seconds", 40},
}

var RareByID = func() map[string]*RareItem {
	m := map[string]*RareItem{}
	for i := range RareItems {
		m[RareItems[i].ID] = &RareItems[i]
	}
	return m
}()

// Ware is one item on the trader's mat.
type Ware struct {
	ID    string
	Price int
	Sold  bool
}

// Trader is the travelling merchant on the map.
type Trader struct {
	X, Y, T float64
	Stock   []Ware
	open    []float64 // per player: seconds standing next to them
	done    []bool    // per player: visited (step away to browse again)
}

// rare: the current player owns rare item id.
func (w *World) rare(id string) bool { return w.Rare[id] }

func (w *World) stepTrader(dt float64) {
	w.trT -= dt
	if w.trT <= 0 {
		w.trT = TraderEvery
		if w.Trader == nil && w.rng.Float64() < TraderP {
			w.spawnTrader()
		}
	}
	tr := w.Trader
	if tr == nil {
		return
	}
	tr.T += dt
	if tr.T > TraderStay || w.nearIn(tr.X, tr.Y, 2200) < 0 {
		w.Trader = nil
		w.message("The trader packed up and moved on.")
		return
	}
	for len(tr.open) < len(w.PL) {
		tr.open, tr.done = append(tr.open, 0), append(tr.done, false)
	}
	for i, c := range w.PL {
		if c.Gone || c.P.Dead {
			continue
		}
		d := dist(c.P.X, c.P.Y, tr.X, tr.Y)
		if d > traderR+40 {
			tr.done[i], tr.open[i] = false, 0
			continue
		}
		if d < traderR && !tr.done[i] && w.PendingKind(i) == "" {
			tr.open[i] += dt
			if tr.open[i] >= traderOpenT {
				tr.done[i] = true
				c.Queue = append(c.Queue, &Prompt{Kind: "shop"})
				w.prompt()
			}
		}
	}
}

// spawnTrader sets the trader up a short walk from a random player, with three rare items.
func (w *World) spawnTrader() {
	w.useP(w.rndAlive())
	a := w.rnd(0, 6.283)
	tr := &Trader{X: w.P.X + math.Cos(a)*320, Y: w.P.Y + math.Sin(a)*320}
	for i := 0; i < 8; i++ { // keep them off obstacles
		x, y := tr.X, tr.Y
		w.push(&x, &y, 30)
		if x == tr.X && y == tr.Y {
			break
		}
		tr.X, tr.Y = x+30, y
	}
	idx := w.rng.Perm(len(RareItems))
	k := 1 + w.T/900 // prices grow with the run (coins do too)
	for _, i := range idx[:3] {
		r := RareItems[i]
		tr.Stock = append(tr.Stock, Ware{ID: r.ID, Price: int(math.Round(r.Price*k/5) * 5)})
	}
	w.Trader = tr
	w.sfx("elite")
	w.message("A travelling trader has set up shop nearby!")
	w.Notes = append(w.Notes, "A travelling trader appeared! Find them before they leave")
}

// ShopFor: player p buys ware i, or leaves with -1 / out of range. The shop stays open after a
// purchase so they can buy more.
func (w *World) ShopFor(p, i int) {
	if w.PendingKind(p) != "shop" {
		return
	}
	w.useP(p)
	tr := w.Trader
	if tr == nil || i < 0 || i >= len(tr.Stock) {
		c := w.PL[p]
		c.Queue = c.Queue[1:]
		w.prompt()
		return
	}
	wr := &tr.Stock[i]
	if wr.Sold || w.Rare[wr.ID] || w.Coins < float64(wr.Price) {
		return
	}
	w.Coins -= float64(wr.Price)
	wr.Sold = true
	w.Rare[wr.ID] = true
	w.RareOrder = append(w.RareOrder, wr.ID)
	w.sfx("chest")
	w.addFX(&FX{Kind: "txt", X: w.P.X, Y: w.P.Y - 40, Life: 2, Text: RareByID[wr.ID].Name + "!", Col: "gold"})
	if wr.ID == "eviolite" {
		add := math.Round(w.P.MHP * .25)
		w.P.MHP += add
		w.P.HP += add
	}
}

// CanBuy: player p can afford ware i and doesn't own it yet.
func (w *World) CanBuy(i int) bool {
	tr := w.Trader
	if tr == nil || i < 0 || i >= len(tr.Stock) {
		return false
	}
	wr := tr.Stock[i]
	return !wr.Sold && !w.Rare[wr.ID] && w.Coins >= float64(wr.Price)
}

// botShop: bots buy the priciest thing they can afford, then leave.
func (w *World) botShop(p int) {
	w.useP(p)
	best := -1
	if tr := w.Trader; tr != nil {
		for i, wr := range tr.Stock {
			if w.CanBuy(i) && (best < 0 || wr.Price > tr.Stock[best].Price) {
				best = i
			}
		}
	}
	if best >= 0 {
		w.ShopFor(p, best)
	}
	w.ShopFor(p, -1)
}

// TestTrader (screenshots): a trader right next to the player and coins to spend.
func (w *World) TestTrader() {
	w.trT = 0
	w.spawnTrader()
	w.Trader.X, w.Trader.Y = w.P.X+70, w.P.Y
	w.Coins += 150
}
