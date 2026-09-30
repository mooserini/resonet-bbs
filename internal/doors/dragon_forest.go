package doors

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"time"

	"wolfbbs/internal/ui"
)

// Forest fights per day, LORD-style. Entering the door costs a door turn;
// each monster hunted in the forest costs one of these.
const dragonForestFightsPerDay = 15

// newDragonRand is swapped out by tests for a seeded source.
var newDragonRand = func() *rand.Rand {
	return rand.New(rand.NewSource(time.Now().UnixNano()))
}

type dragonMonster struct {
	Name     string
	Weapon   string
	HP       int
	MaxHP    int
	Strength int
	Defense  int
	Gold     int64
	XP       int64
}

type dragonMonsterTemplate struct {
	Name   string
	Weapon string
	// Scale is a percentage applied to the tier's base stats so each tier has
	// a pushover, a couple of regulars and one that hits hard.
	Scale int
}

// One row per hero level; heroes past the last row meet the last row's
// monsters with stats still scaled to their level.
var dragonMonsterTiers = [][]dragonMonsterTemplate{
	{{"Bog Rat", "yellowed teeth", 80}, {"Mossback Toad", "sticky tongue", 90}, {"Lost Scarecrow", "rusty sickle", 100}, {"Hedge Goblin", "sharpened spoon", 115}},
	{{"Thornwolf", "bramble fangs", 85}, {"Mad Woodcutter", "notched axe", 100}, {"Giant Beetle", "iron mandibles", 105}, {"Swamp Hag", "boiling ladle", 120}},
	{{"Cave Imp", "stolen dagger", 85}, {"Barrow Wight", "grave chill", 100}, {"Highway Brigand", "short sword", 105}, {"Dire Boar", "curved tusks", 120}},
	{{"Will-o'-Wisp", "blinding light", 85}, {"Orc Raider", "spiked club", 100}, {"Bramble Troll", "uprooted sapling", 110}, {"Fen Serpent", "venom bite", 120}},
	{{"Ghoul", "filthy claws", 90}, {"Hill Ogre", "stone maul", 105}, {"Dark Acolyte", "cursed staff", 100}, {"Cockatrice", "petrifying glare", 120}},
	{{"Wraith Knight", "spectral lance", 95}, {"Owlbear", "crushing hug", 110}, {"Gnoll Warlord", "flail", 100}, {"Manticore", "tail spikes", 120}},
	{{"Stone Golem", "granite fists", 100}, {"Minotaur", "labrys", 110}, {"Harpy Queen", "shrieking talons", 95}, {"Basilisk", "deadly gaze", 120}},
	{{"Frost Giant", "glacier hammer", 105}, {"Vampire Count", "draining kiss", 110}, {"Chimera", "three heads", 115}, {"Wyvern", "barbed tail", 120}},
	{{"Lich Apprentice", "bone wand", 100}, {"Hydra", "snapping jaws", 115}, {"Death Knight", "black greatsword", 110}, {"Beholder", "eye rays", 125}},
	{{"Fire Elemental", "living flame", 105}, {"Rakshasa", "illusions", 110}, {"Storm Giant", "lightning spear", 115}, {"Iron Dragon", "molten breath", 125}},
	{{"Pit Fiend", "hellfire whip", 110}, {"Elder Lich", "soul drain", 115}, {"Kraken Spawn", "crushing tentacles", 120}, {"Shadow Titan", "void blade", 125}},
	{{"Archdemon", "burning glaive", 115}, {"Ancient Wyrm", "doom breath", 125}, {"Fallen Seraph", "broken halo", 120}, {"The Nameless Knight", "a sword with no name", 130}},
}

func dragonSpawnMonster(level int, rng *rand.Rand) dragonMonster {
	if level < 1 {
		level = 1
	}
	tier := dragonMonsterTiers[minInt(level, len(dragonMonsterTiers))-1]
	tpl := tier[rng.Intn(len(tier))]
	scale := func(base int) int {
		return maxInt(1, base*tpl.Scale/100)
	}
	hp := scale(10 + 6*(level-1) + rng.Intn(3+level))
	return dragonMonster{
		Name:     tpl.Name,
		Weapon:   tpl.Weapon,
		HP:       hp,
		MaxHP:    hp,
		Strength: scale(7 + 3*(level-1)),
		Defense:  (level - 1) / 2,
		Gold:     int64(scale(8*level + rng.Intn(10*level+1))),
		XP:       int64(scale(6 + 8*level + rng.Intn(4*level+1))),
	}
}

// dragonRollHit returns base/2 + a random roll up to base/2, less the target's
// defense, never below zero.
func dragonRollHit(base, defense int, rng *rand.Rand) int {
	half := maxInt(1, base/2)
	return maxInt(0, half+rng.Intn(half+1)-defense)
}

func dragonResetForestDay(state *dragonTavernState, now time.Time) {
	if state == nil {
		return
	}
	day := now.UTC().Format("2006-01-02")
	if state.ForestDay == day {
		return
	}
	state.ForestDay = day
	state.ForestFightsUsed = 0
	if state.Dead {
		state.Dead = false
		state.HP = state.MaxHP
	}
}

func dragonForestFightsLeft(state dragonTavernState) int {
	return maxInt(0, dragonForestFightsPerDay-state.ForestFightsUsed)
}

func dragonHealCost(state dragonTavernState) int64 {
	return int64(maxInt(0, state.MaxHP-state.HP) * maxInt(1, state.Level))
}

// dragonForest runs the forest until the player heads back to town or dies.
// save is called after every fight so a dropped connection mid-session never
// loses a kill or refunds a death.
func dragonForest(reader *bufio.Reader, stdout io.Writer, state *dragonTavernState, rng *rand.Rand, save func() error) error {
	for {
		dragonResetForestDay(state, time.Now().UTC())
		if state.Dead {
			return nil
		}
		lines := []string{
			"The trees close in around you. Something is watching.",
			"",
			fmt.Sprintf("HP: %d/%d   Gold: %d   Forest fights left today: %d", state.HP, state.MaxHP, state.Gold, dragonForestFightsLeft(*state)),
			"",
			"(L)ook for something to kill",
			fmt.Sprintf("(H)ealer's hut  [%d gold to heal fully]", dragonHealCost(*state)),
			"(R)eturn to town",
			"",
			"Your command:",
		}
		renderDoorPanel(stdout, "The Forest", lines, ui.FgGreen)
		key, err := readDoorKey(reader)
		if err != nil {
			return err
		}
		switch key {
		case "R", "Q", "ESC":
			return nil
		case "H":
			io.WriteString(stdout, "\r\n"+dragonHealer(state))
			if err := save(); err != nil {
				return err
			}
			pauseDoor(reader, stdout)
		case "L":
			if dragonForestFightsLeft(*state) <= 0 {
				io.WriteString(stdout, "\r\nYou are too tired to fight any more today. Come back tomorrow.\r\n")
				pauseDoor(reader, stdout)
				continue
			}
			state.ForestFightsUsed++
			state.ForestRuns++
			if err := dragonForestEncounter(reader, stdout, state, rng); err != nil {
				return err
			}
			dragonNormalizeStats(state)
			if err := save(); err != nil {
				return err
			}
			pauseDoor(reader, stdout)
		}
	}
}

func dragonHealer(state *dragonTavernState) string {
	if state.HP >= state.MaxHP {
		return "The healer looks you over. \"You're fine. Get out.\"\r\n"
	}
	cost := dragonHealCost(*state)
	if state.Gold >= cost {
		state.Gold -= cost
		state.HP = state.MaxHP
		return fmt.Sprintf("The healer patches you up for %d gold. HP %d/%d.\r\n", cost, state.HP, state.MaxHP)
	}
	perHP := int64(maxInt(1, state.Level))
	affordable := int(state.Gold / perHP)
	if affordable <= 0 {
		return "\"No gold, no healing,\" the healer grunts.\r\n"
	}
	state.Gold -= int64(affordable) * perHP
	state.HP += affordable
	return fmt.Sprintf("You can only afford %d HP of healing. HP %d/%d.\r\n", affordable, state.HP, state.MaxHP)
}

// dragonForestEncounter plays out one forest event: usually a monster fight,
// sometimes a find.
func dragonForestEncounter(reader *bufio.Reader, stdout io.Writer, state *dragonTavernState, rng *rand.Rand) error {
	io.WriteString(stdout, ui.ClearScreen())
	switch roll := rng.Intn(100); {
	case roll < 6:
		gold := int64(10*state.Level + rng.Intn(20*state.Level+1))
		state.Gold += gold
		io.WriteString(stdout, ui.Color(ui.FgYellow, "", fmt.Sprintf("You trip over a rotting pack. Inside: %d gold!", gold))+"\r\n")
		return nil
	case roll < 10:
		heal := state.MaxHP - state.HP
		state.HP = state.MaxHP
		io.WriteString(stdout, ui.Color(ui.FgCyan, "", "You find a clear spring and drink deeply.")+"\r\n")
		if heal > 0 {
			io.WriteString(stdout, fmt.Sprintf("You feel refreshed. +%d HP.\r\n", heal))
		} else {
			io.WriteString(stdout, "Tasty, but you were already in fine shape.\r\n")
		}
		return nil
	}

	m := dragonSpawnMonster(state.Level, rng)
	io.WriteString(stdout, ui.Color(ui.Bold+ui.FgRed, "", "**  You have encountered "+m.Name+"!  **")+"\r\n")
	io.WriteString(stdout, fmt.Sprintf("It comes at you with %s.\r\n", m.Weapon))
	for {
		io.WriteString(stdout, fmt.Sprintf("\r\nYour HP: %s   %s's HP: %s\r\n",
			ui.Color(ui.FgGreen, "", fmt.Sprintf("%d/%d", state.HP, state.MaxHP)),
			m.Name,
			ui.Color(ui.FgRed, "", fmt.Sprintf("%d", m.HP))))
		io.WriteString(stdout, "(A)ttack  (S)tats  (R)un\r\nYour command: ")
		key, err := readDoorKey(reader)
		if err != nil {
			return err
		}
		switch key {
		case "S":
			io.WriteString(stdout, fmt.Sprintf("\r\nLevel %d  ATK %d  DEF %d  Gold %d  XP %d\r\n", state.Level, state.Attack, state.Defense, state.Gold, state.XP))
			continue
		case "R":
			if rng.Intn(100) < 60 {
				io.WriteString(stdout, "\r\nYou crash through the undergrowth and get away.\r\n")
				return nil
			}
			io.WriteString(stdout, "\r\n"+ui.Color(ui.FgRed, "", fmt.Sprintf("%s blocks your escape!", m.Name))+"\r\n")
		case "A":
			dmg := dragonRollHit(state.Attack, m.Defense, rng)
			if dmg > 0 && rng.Intn(100) < 10 {
				dmg *= 2
				io.WriteString(stdout, "\r\n"+ui.Color(ui.Bold+ui.FgYellow, "", "**  POWER MOVE  **"))
			}
			if dmg <= 0 {
				io.WriteString(stdout, fmt.Sprintf("\r\nYou swing at %s and miss.\r\n", m.Name))
			} else {
				m.HP -= dmg
				io.WriteString(stdout, fmt.Sprintf("\r\nYou hit %s for %s damage!\r\n", m.Name, ui.Color(ui.FgYellow, "", fmt.Sprintf("%d", dmg))))
			}
			if m.HP <= 0 {
				state.Gold += m.Gold
				state.XP += m.XP
				io.WriteString(stdout, "\r\n"+ui.Color(ui.Bold+ui.FgGreen, "", fmt.Sprintf("You have killed %s!", m.Name))+"\r\n")
				io.WriteString(stdout, fmt.Sprintf("You find %d gold and gain %d experience.\r\n", m.Gold, m.XP))
				if ups := dragonApplyLevelUps(state); ups > 0 {
					io.WriteString(stdout, ui.Color(ui.Bold+ui.FgMagenta, "", fmt.Sprintf("You feel stronger! You are now level %d.", state.Level))+"\r\n")
				}
				return nil
			}
		default:
			continue
		}

		hit := dragonRollHit(m.Strength, state.Defense, rng)
		if hit <= 0 {
			io.WriteString(stdout, fmt.Sprintf("%s lunges with %s and misses.\r\n", m.Name, m.Weapon))
			continue
		}
		state.HP -= hit
		io.WriteString(stdout, fmt.Sprintf("%s hits you with %s for %s damage!\r\n", m.Name, m.Weapon, ui.Color(ui.FgRed, "", fmt.Sprintf("%d", hit))))
		if state.HP <= 0 {
			dragonDie(stdout, state, m.Name)
			return nil
		}
	}
}

// dragonDie applies the classic penalty: gold on hand is lost, a tenth of
// your experience goes with it, and you stay dead until tomorrow.
func dragonDie(stdout io.Writer, state *dragonTavernState, killer string) {
	lostGold := state.Gold
	lostXP := state.XP / 10
	state.Gold = 0
	state.XP -= lostXP
	state.HP = 0
	state.Dead = true
	state.Deaths++
	io.WriteString(stdout, "\r\n"+ui.Color(ui.Bold+ui.FgRed, "", fmt.Sprintf("You have been slain by %s.", killer))+"\r\n")
	io.WriteString(stdout, fmt.Sprintf("You lose %d gold and %d experience.\r\nYou may continue your adventures tomorrow.\r\n", lostGold, lostXP))
}
