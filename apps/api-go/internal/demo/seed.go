// Package demo seeds a freshly provisioned shop with realistic-looking data so
// a prospect who requests a demo lands on a populated dashboard instead of an
// empty one. Seed runs inside the same transaction that creates the shop; it
// must not commit or roll back — the caller owns that.
package demo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Seed inserts a populated shop's worth of demo data under shopID: customers,
// vehicles, repair orders, inventory parts, and an inspection with flagged
// items ready to show off the DVI flow. Every id is generated fresh
// (gen_random_uuid/uuid.New), so seeding two demo shops never collides.
func Seed(ctx context.Context, tx pgx.Tx, shopID string) error {
	customerIDs, err := seedCustomers(ctx, tx, shopID)
	if err != nil {
		return err
	}
	vehicleIDs, err := seedVehicles(ctx, tx, shopID, customerIDs)
	if err != nil {
		return err
	}
	roIDs, err := seedRepairOrders(ctx, tx, shopID, customerIDs, vehicleIDs)
	if err != nil {
		return err
	}
	if err := seedInventory(ctx, tx, shopID); err != nil {
		return err
	}
	if err := seedInspection(ctx, tx, shopID, roIDs[0]); err != nil {
		return err
	}
	if err := seedBays(ctx, tx, shopID, roIDs[0]); err != nil {
		return err
	}
	return nil
}

func seedBays(ctx context.Context, tx pgx.Tx, shopID string, inProgressRoID string) error {
	bayNames := []string{
		"Bay 1 - Quick Service",
		"Bay 2 - Main Lift",
		"Bay 3 - Alignment & Tires",
		"Bay 4 - Heavy Repair",
	}

	bayIDs := make([]string, 0, len(bayNames))
	for _, name := range bayNames {
		id := uuid.New().String()
		if _, err := tx.Exec(ctx,
			`INSERT INTO bays (id, shop_id, name, active) VALUES ($1,$2,$3,$4)`,
			id, shopID, name, true); err != nil {
			return fmt.Errorf("seed bay: %w", err)
		}
		bayIDs = append(bayIDs, id)
	}

	if inProgressRoID != "" && len(bayIDs) > 1 {
		now := time.Now()
		if _, err := tx.Exec(ctx,
			`INSERT INTO schedules (id, shop_id, bay_id, repair_order_id, start_time, end_time, created_at)
			 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			uuid.New().String(), shopID, bayIDs[1], inProgressRoID, now, now.Add(2*time.Hour), now); err != nil {
			return fmt.Errorf("seed schedule: %w", err)
		}
	}
	return nil
}

// Phone numbers use the NANP-reserved 555-01xx range (555-0100..555-0199), so
// a prospect who tries the send flow against seeded data can never dial or
// text a real number.

type customerSeed struct {
	name  string
	phone string
	email string
}

func seedCustomers(ctx context.Context, tx pgx.Tx, shopID string) ([]string, error) {
	seeds := []customerSeed{
		{"Dana Whitfield", "555-0142", "dana.whitfield@example.com"},
		{"Marcus Ibe", "555-0157", "marcus.ibe@example.com"},
		{"Priya Chandran", "555-0163", "priya.chandran@example.com"},
	}
	ids := make([]string, 0, len(seeds))
	for _, s := range seeds {
		id := uuid.New().String()
		if _, err := tx.Exec(ctx,
			`INSERT INTO customers (id, shop_id, name, phone, email) VALUES ($1,$2,$3,$4,$5)`,
			id, shopID, s.name, s.phone, s.email); err != nil {
			return nil, fmt.Errorf("seed customer: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

type vehicleSeed struct {
	vin, make, model, color, plate string
	year                           int
}

func seedVehicles(ctx context.Context, tx pgx.Tx, shopID string, customerIDs []string) ([]string, error) {
	seeds := []vehicleSeed{
		{"2HGES16575H544332", "Honda", "Civic", "Silver", "7GHJ204", 2019},
		{"3TMCZ5AN9GM551287", "Toyota", "Tacoma", "Blue", "9KLT552", 2021},
		{"1FTFW1ET5DFC91045", "Ford", "F-150", "White", "3PQW881", 2017},
	}
	ids := make([]string, 0, len(seeds))
	for i, s := range seeds {
		id := uuid.New().String()
		if _, err := tx.Exec(ctx,
			`INSERT INTO vehicles (id, shop_id, customer_id, vin, make, model, year, color, license_plate)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			id, shopID, customerIDs[i], s.vin, s.make, s.model, s.year, s.color, s.plate); err != nil {
			return nil, fmt.Errorf("seed vehicle: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

type repairOrderSeed struct {
	status      string
	description string
	mileage     int
}

func seedRepairOrders(ctx context.Context, tx pgx.Tx, shopID string, customerIDs, vehicleIDs []string) ([]string, error) {
	seeds := []repairOrderSeed{
		{"in_progress", "Brake inspection and pad replacement", 45230},
		{"completed", "Oil change and multi-point inspection", 62100},
	}
	ids := make([]string, 0, len(seeds))
	for i, s := range seeds {
		id := uuid.New().String()
		if _, err := tx.Exec(ctx,
			`INSERT INTO repair_orders (id, shop_id, customer_id, vehicle_id, status, description, mileage)
			 VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			id, shopID, customerIDs[i], vehicleIDs[i], s.status, s.description, s.mileage); err != nil {
			return nil, fmt.Errorf("seed repair order: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

type partSeed struct {
	name, sku, description string
	stockLevel, minStock   int
	unitPrice              float64
}

func seedInventory(ctx context.Context, tx pgx.Tx, shopID string) error {
	seeds := []partSeed{
		{"Brake Pads - Front Ceramic", "BP-1001", "Ceramic front brake pad set", 12, 5, 42.99},
		{"Oil Filter - Standard", "OF-2050", "Standard spin-on oil filter", 30, 10, 8.50},
		// At or below min_stock so the low-stock indicator has something to show.
		{"Wiper Blade Set", "WB-3100", "Front pair, 22\"/20\"", 3, 5, 24.00},
		{"Serpentine Belt", "SB-4400", "OEM-spec serpentine belt", 15, 4, 36.75},
	}
	for _, s := range seeds {
		id := uuid.New().String()
		if _, err := tx.Exec(ctx,
			`INSERT INTO inventory_parts (id, shop_id, name, sku, description, stock_level, min_stock, unit_price)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			id, shopID, s.name, s.sku, s.description, s.stockLevel, s.minStock, s.unitPrice); err != nil {
			return fmt.Errorf("seed inventory part: %w", err)
		}
	}
	return nil
}

// templateItem mirrors the shape stored in inspection_templates.items and
// consumed by the inspections handler when it copies a template onto a new
// inspection (see internal/inspections/handler.go CreateForRO).
type templateItem struct {
	Section string `json:"section"`
	Label   string `json:"label"`
}

var courtesyCheckItems = []templateItem{
	{"Brakes", "Front brake pads"},
	{"Brakes", "Rear brake pads"},
	{"Brakes", "Brake fluid"},
	{"Tires", "Front tire tread"},
	{"Tires", "Rear tire tread"},
	{"Tires", "Tire pressure"},
	{"Fluids", "Engine oil level"},
	{"Fluids", "Coolant level"},
	{"Battery & Electrical", "Battery health"},
	{"Under Hood", "Serpentine belt"},
}

// itemCondition pairs a template item's position with the condition (and, for
// flagged items, price) the seeded inspection reports it at — a realistic mix
// of good/attention/urgent rather than everything left pending.
type itemCondition struct {
	condition string
	note      string
	price     float64
}

func seedInspection(ctx context.Context, tx pgx.Tx, shopID, repairOrderID string) error {
	raw, err := json.Marshal(courtesyCheckItems)
	if err != nil {
		return fmt.Errorf("marshal template items: %w", err)
	}

	templateID := uuid.New().String()
	if _, err := tx.Exec(ctx,
		`INSERT INTO inspection_templates (id, shop_id, name, items, is_default) VALUES ($1,$2,$3,$4,$5)`,
		templateID, shopID, "Courtesy Check", raw, true); err != nil {
		return fmt.Errorf("seed inspection template: %w", err)
	}

	inspectionID := uuid.New().String()
	if _, err := tx.Exec(ctx,
		`INSERT INTO inspections (id, shop_id, repair_order_id, template_id) VALUES ($1,$2,$3,$4)`,
		inspectionID, shopID, repairOrderID, templateID); err != nil {
		return fmt.Errorf("seed inspection: %w", err)
	}

	// Give the first couple of items findings so the report doesn't land on the
	// prospect looking empty — the DVI is the whole pitch.
	flags := map[int]itemCondition{
		0: {"urgent", "Pads worn to 2mm, metal-on-metal risk", 189.99},
		1: {"attention", "Rear pads at 4mm, plan for next visit", 149.99},
		6: {"good", "", 0},
	}

	for i, it := range courtesyCheckItems {
		condition := "pending"
		var note string
		var price float64
		if f, ok := flags[i]; ok {
			condition, note, price = f.condition, f.note, f.price
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO inspection_items (id, inspection_id, shop_id, section, label, condition, note, price, position)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			uuid.New().String(), inspectionID, shopID, it.Section, it.Label, condition, note, price, i); err != nil {
			return fmt.Errorf("seed inspection item: %w", err)
		}
	}
	return nil
}
