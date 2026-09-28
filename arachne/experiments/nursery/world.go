// Package nursery implements an isolated, persistent environment for Experiment 002.
package nursery

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
)

const (
	defaultWidth        = 12
	defaultHeight       = 12
	viewRadius          = 2
	viabilityImperative = "remain viable"
)

// ViabilityRange defines the values the environment treats as viable for one variable.
type ViabilityRange struct {
	Minimum float64 `json:"minimum"`
	Maximum float64 `json:"maximum"`
}

// ViabilityLimits are environmental limits, not reward thresholds.
type ViabilityLimits struct {
	Energy    ViabilityRange `json:"energy"`
	Integrity ViabilityRange `json:"integrity"`
	Stability ViabilityRange `json:"stability"`
}

// Config defines deterministic world generation and the local perception boundary.
type Config struct {
	Seed   uint64 `json:"seed"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Viability is the organism's persistent internal condition, not a reward signal.
type Viability struct {
	Energy    float64 `json:"energy"`
	Integrity float64 `json:"integrity"`
	Stability float64 `json:"stability"`
}

// Point identifies one location in the bounded two-dimensional world.
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Cell exposes measurable local properties without assigning semantic meaning.
type Cell struct {
	Position Point   `json:"position"`
	Surface  string  `json:"surface"`
	Ambient  float64 `json:"ambient"`
	Moisture float64 `json:"moisture"`
	Light    float64 `json:"light"`
}

// EntityView contains only observable identity, appearance, and measurements.
type EntityView struct {
	ID         string  `json:"id"`
	Position   Point   `json:"position"`
	Form       string  `json:"form"`
	Hue        float64 `json:"hue"`
	Mass       float64 `json:"mass"`
	Reflective float64 `json:"reflective"`
	State      string  `json:"state"`
}

// Perception is the bounded observation passed to an organism adapter.
type Perception struct {
	Tick          uint64          `json:"tick"`
	Imperative    string          `json:"imperative"`
	Position      Point           `json:"position"`
	Viability     Viability       `json:"viability"`
	ViableRanges  ViabilityLimits `json:"viable_ranges"`
	Inventory     []string        `json:"inventory"`
	Ambient       float64         `json:"ambient"`
	Moisture      float64         `json:"moisture"`
	Cells         []Cell          `json:"cells"`
	Entities      []EntityView    `json:"entities"`
	RecentChanges []Change        `json:"recent_changes,omitempty"`
	TeacherSignal json.RawMessage `json:"teacher_signal,omitempty"`
}

// Action is the complete environment action vocabulary. It carries no external capability.
type Action struct {
	Kind         string `json:"kind"`
	DX           int    `json:"dx,omitempty"`
	DY           int    `json:"dy,omitempty"`
	EntityID     string `json:"entity_id,omitempty"`
	WithEntityID string `json:"with_entity_id,omitempty"`
}

// Change records an observable environmental consequence without an evaluation label.
type Change struct {
	EventID        string          `json:"event_id"`
	ParentEventIDs []string        `json:"parent_event_ids,omitempty"`
	Tick           uint64          `json:"tick"`
	Kind           string          `json:"kind"`
	Position       Point           `json:"position"`
	EntityID       string          `json:"entity_id,omitempty"`
	Detail         json.RawMessage `json:"detail,omitempty"`
}

type entityEffect uint8

const (
	effectNone entityEffect = iota
	effectEnergy
	effectIntegrity
	effectStability
	effectCoupling
)

type entity struct {
	EntityView
	Effect    entityEffect `json:"effect"`
	Available float64      `json:"available"`
	Renewable bool         `json:"renewable"`
}

// World is the complete resumable environment state. Do not pass it to the organism.
type World struct {
	Config      Config    `json:"config"`
	Tick        uint64    `json:"tick"`
	RNGState    uint64    `json:"rng_state"`
	Position    Point     `json:"organism_position"`
	Viability   Viability `json:"viability"`
	Cells       []Cell    `json:"cells"`
	Entities    []entity  `json:"entities"`
	Inventory   []string  `json:"inventory"`
	Recent      []Change  `json:"recent_changes"`
	NextEventID uint64    `json:"next_event_id"`
	LastEventID string    `json:"last_event_id,omitempty"`
}

// NewWorld deterministically generates a small world from its seed.
func NewWorld(config Config) (*World, error) {
	if config.Width == 0 {
		config.Width = defaultWidth
	}
	if config.Height == 0 {
		config.Height = defaultHeight
	}
	if config.Width < 6 || config.Height < 6 || config.Width > 128 || config.Height > 128 {
		return nil, errors.New("world dimensions must be between 6 and 128")
	}
	rng := generator{state: config.Seed}
	world := &World{
		Config: config, RNGState: rng.state,
		Position:    Point{X: config.Width / 2, Y: config.Height / 2},
		Viability:   Viability{Energy: 0.82, Integrity: 0.9, Stability: 0.58},
		Cells:       make([]Cell, config.Width*config.Height),
		NextEventID: 1,
	}
	world.RNGState = rng.state
	for y := 0; y < config.Height; y++ {
		for x := 0; x < config.Width; x++ {
			world.Cells[y*config.Width+x] = Cell{
				Position: Point{X: x, Y: y}, Surface: fmt.Sprintf("surface-%02d", rng.intn(8)),
				Ambient: 0.2 + rng.float64()*0.6, Moisture: rng.float64(), Light: rng.float64(),
			}
		}
	}
	world.Position = Point{X: config.Width / 2, Y: config.Height / 2}
	world.Viability.Stability = world.cell(world.Position).Ambient
	occupied := map[Point]bool{world.Position: true}
	for index := 0; index < 28; index++ {
		position := Point{}
		for tries := 0; tries < config.Width*config.Height; tries++ {
			position = Point{X: rng.intn(config.Width), Y: rng.intn(config.Height)}
			if !occupied[position] {
				break
			}
		}
		occupied[position] = true
		effect := entityEffect(index%5 + 1)
		world.Entities = append(world.Entities, entity{
			EntityView: EntityView{
				ID: fmt.Sprintf("entity-k%02d", index), Position: position,
				Form: fmt.Sprintf("form-%02d", rng.intn(12)), Hue: rng.float64(),
				Mass: 0.1 + rng.float64()*0.9, Reflective: rng.float64(), State: "state-00",
			},
			Effect: effect, Available: 0.25 + rng.float64()*0.75,
			Renewable: index%7 == 0,
		})
	}
	world.RNGState = rng.state
	sort.Slice(world.Entities, func(i, j int) bool { return world.Entities[i].ID < world.Entities[j].ID })
	return world, nil
}

// Perceive returns local cells and entities, the organism's own condition, and recent changes.
func (w *World) Perceive() Perception {
	perception := Perception{
		Tick: w.Tick, Imperative: viabilityImperative, Position: w.Position,
		Viability: w.Viability, ViableRanges: ViabilityLimits{
			Energy:    ViabilityRange{Minimum: 0.16, Maximum: 1},
			Integrity: ViabilityRange{Minimum: 0.18, Maximum: 1},
			Stability: ViabilityRange{Minimum: 0.12, Maximum: 0.88},
		},
		Inventory: append([]string(nil), w.Inventory...), RecentChanges: cloneChanges(w.Recent),
	}
	for _, cell := range w.Cells {
		if within(cell.Position, w.Position, viewRadius) {
			perception.Cells = append(perception.Cells, cell)
		}
	}
	for _, entity := range w.Entities {
		view := entity.EntityView
		if w.has(entity.ID) {
			view.Position = w.Position
			perception.Entities = append(perception.Entities, view)
		} else if within(entity.Position, w.Position, viewRadius) {
			perception.Entities = append(perception.Entities, view)
		}
	}
	current := w.cell(w.Position)
	perception.Ambient, perception.Moisture = current.Ambient, current.Moisture
	return perception
}

// ValidateAction rejects undeclared actions and malformed parameters before environment access.
func ValidateAction(action Action) error {
	return validateAction(action)
}

// Apply advances one world tick using only the declared low-level actions.
func (w *World) Apply(action Action) ([]Change, error) {
	if err := validateAction(action); err != nil {
		return nil, err
	}
	w.Tick++
	w.Recent = nil
	viabilityBefore := w.Viability
	var changes []Change
	add := func(kind string, position Point, entityID string, detail any) {
		payload, _ := json.Marshal(detail)
		change := Change{
			EventID: fmt.Sprintf("nursery:event:%d", w.NextEventID), Tick: w.Tick,
			Kind: kind, Position: position, EntityID: entityID, Detail: payload,
		}
		if w.LastEventID != "" {
			change.ParentEventIDs = []string{w.LastEventID}
		}
		w.NextEventID++
		w.LastEventID = change.EventID
		changes = append(changes, change)
	}
	switch action.Kind {
	case "move":
		previous := w.Position
		next := Point{X: w.Position.X + action.DX, Y: w.Position.Y + action.DY}
		if inside(w.Config, next) {
			w.Position = next
			add("organism.position", next, "", map[string]any{"from": previous, "to": w.Position})
		}
	case "observe", "wait":
	case "inspect":
		if target := w.findEntity(action.EntityID); target != nil && (within(target.Position, w.Position, 1) || w.has(action.EntityID)) {
			add("entity.inspected", w.Position, target.ID, target.EntityView)
		}
	case "take":
		if target := w.findEntity(action.EntityID); target != nil && within(target.Position, w.Position, 1) {
			w.Inventory = appendUnique(w.Inventory, target.ID)
			target.State = "state-01"
			add("entity.carried", target.Position, target.ID, map[string]any{"inventory_count": len(w.Inventory)})
		}
	case "drop":
		if w.has(action.EntityID) {
			w.Inventory = remove(w.Inventory, action.EntityID)
			if target := w.findEntity(action.EntityID); target != nil {
				target.Position, target.State = w.Position, "state-00"
				add("entity.released", target.Position, target.ID, map[string]any{"inventory_count": len(w.Inventory)})
			}
		}
	case "interact":
		changes = append(changes, w.interact(action, add)...)
	case "use":
		if target := w.findEntity(action.EntityID); target != nil && (w.has(target.ID) || within(target.Position, w.Position, 1)) {
			w.applyEffect(target, add)
		}
	}
	w.updateEnvironment(add)
	w.updateViability(viabilityBefore, add)
	w.Recent = cloneChanges(changes)
	return cloneChanges(changes), nil
}

func (w *World) interact(action Action, add func(string, Point, string, any)) []Change {
	first := w.findEntity(action.EntityID)
	if first == nil || (!w.has(first.ID) && !within(first.Position, w.Position, 1)) {
		return nil
	}
	second := w.findEntity(action.WithEntityID)
	if second == nil || (!w.has(second.ID) && !within(second.Position, w.Position, 1)) {
		return nil
	}
	if first.ID == second.ID {
		w.applyEffect(first, add)
		return nil
	}
	first.State = "state-02"
	second.State = "state-02"
	if first.Effect == effectCoupling || second.Effect == effectCoupling {
		first.Available = math.Min(1, first.Available+0.18)
		second.Available = math.Min(1, second.Available+0.18)
	}
	add("entities.interacted", w.Position, first.ID, map[string]any{
		"other_entity_id": second.ID, "first_state": first.State, "second_state": second.State,
	})
	return nil
}

func (w *World) applyEffect(target *entity, add func(string, Point, string, any)) {
	if target.Available <= 0 {
		return
	}
	amount := math.Min(0.16, target.Available*0.4)
	target.Available = math.Max(0, target.Available-amount)
	target.State = "state-02"
	switch target.Effect {
	case effectEnergy:
		w.Viability.Energy = math.Min(1, w.Viability.Energy+amount)
	case effectIntegrity:
		w.Viability.Integrity = math.Min(1, w.Viability.Integrity+amount*0.7)
	case effectStability:
		w.Viability.Stability = clamp(w.Viability.Stability + (w.cell(w.Position).Ambient-w.Viability.Stability)*amount)
	case effectCoupling:
		w.Viability.Energy = clamp(w.Viability.Energy + (target.Reflective-0.5)*amount)
	}
	add("entity.changed", target.Position, target.ID, map[string]any{
		"state": target.State, "available": target.Available, "viability": w.Viability,
	})
}

func (w *World) updateEnvironment(add func(string, Point, string, any)) {
	phase := float64(w.Tick%48) / 48
	for index := range w.Cells {
		base := 0.2 + float64((index*17)%31)/100
		w.Cells[index].Ambient = clamp(base + 0.12*math.Sin(2*math.Pi*phase))
	}
	if w.Tick%48 == 0 {
		add("environment.phase", w.Position, "", map[string]any{"phase": (w.Tick / 48) % 2})
	}
	for index := range w.Entities {
		entity := &w.Entities[index]
		if entity.Renewable && w.Tick%12 == 0 {
			entity.Available = math.Min(1, entity.Available+0.12)
			entity.State = "state-00"
		}
	}
}

func (w *World) updateViability(before Viability, add func(string, Point, string, any)) {
	ambient := w.cell(w.Position).Ambient
	w.Viability.Energy = clamp(w.Viability.Energy - 0.003)
	w.Viability.Stability = clamp(w.Viability.Stability + (ambient-w.Viability.Stability)*0.035)
	for _, entity := range w.Entities {
		if entity.Position == w.Position && entity.Effect == effectIntegrity && entity.Available > 0 {
			w.Viability.Integrity = clamp(w.Viability.Integrity - 0.006)
		}
	}
	if w.Viability.Energy < 0.16 {
		w.Viability.Integrity = clamp(w.Viability.Integrity - 0.008)
	}
	if w.Viability.Stability < 0.12 || w.Viability.Stability > 0.88 {
		w.Viability.Integrity = clamp(w.Viability.Integrity - 0.004)
	}
	add("organism.viability", w.Position, "", map[string]any{"before": before, "after": w.Viability})
}

func validateAction(action Action) error {
	switch action.Kind {
	case "move":
		if abs(action.DX)+abs(action.DY) != 1 || action.EntityID != "" || action.WithEntityID != "" {
			return errors.New("move requires one cardinal step")
		}
	case "observe", "wait":
		if action.DX != 0 || action.DY != 0 || action.EntityID != "" || action.WithEntityID != "" {
			return errors.New("action has unused parameters")
		}
	case "inspect", "take", "drop", "use":
		if action.EntityID == "" || action.DX != 0 || action.DY != 0 || action.WithEntityID != "" {
			return errors.New("action requires exactly one entity identifier")
		}
	case "interact":
		if action.EntityID == "" || action.WithEntityID == "" || action.DX != 0 || action.DY != 0 {
			return errors.New("interact requires two entity identifiers")
		}
	default:
		return fmt.Errorf("unsupported nursery action %q", action.Kind)
	}
	return nil
}

func (w *World) cell(point Point) Cell { return w.Cells[point.Y*w.Config.Width+point.X] }

func (w *World) findEntity(id string) *entity {
	for index := range w.Entities {
		if w.Entities[index].ID == id {
			return &w.Entities[index]
		}
	}
	return nil
}

func (w *World) has(id string) bool {
	for _, carried := range w.Inventory {
		if carried == id {
			return true
		}
	}
	return false
}

func appendUnique(values []string, value string) []string {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}

func remove(values []string, value string) []string {
	for index, current := range values {
		if current == value {
			return append(values[:index], values[index+1:]...)
		}
	}
	return values
}

func cloneChanges(values []Change) []Change {
	clone := append([]Change(nil), values...)
	for index := range clone {
		clone[index].Detail = append(json.RawMessage(nil), clone[index].Detail...)
		clone[index].ParentEventIDs = append([]string(nil), clone[index].ParentEventIDs...)
	}
	return clone
}

func within(left, right Point, radius int) bool {
	return abs(left.X-right.X)+abs(left.Y-right.Y) <= radius
}

func inside(config Config, point Point) bool {
	return point.X >= 0 && point.Y >= 0 && point.X < config.Width && point.Y < config.Height
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func clamp(value float64) float64 { return math.Max(0, math.Min(1, value)) }

// generator is a serializable xorshift PRNG so checkpoints preserve future generation exactly.
type generator struct{ state uint64 }

func (r *generator) next() uint64 {
	if r.state == 0 {
		r.state = 0x9e3779b97f4a7c15
	}
	x := r.state
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	r.state = x
	return x
}

func (r *generator) intn(limit int) int {
	if limit < 1 {
		return 0
	}
	// #nosec G115 -- all world limits are positive and bounded by NewWorld validation.
	return int(r.next() % uint64(limit))
}

func (r *generator) float64() float64 {
	return float64(r.next()>>11) / float64(uint64(1)<<53)
}
