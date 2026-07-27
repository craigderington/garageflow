package types

import "time"

type Role string

const (
	RoleOwner         Role = "owner"
	RoleAdmin         Role = "admin"
	RoleServiceWriter Role = "service_writer"
	RoleTechnician    Role = "technician"
)

type User struct {
	ID        string    `json:"id"`
	ShopID    string    `json:"shop_id"`
	Email     string    `json:"email"`
	Role      Role      `json:"role"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Customer struct {
	ID        string    `json:"id"`
	ShopID    string    `json:"shop_id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Vehicle struct {
	ID           string    `json:"id"`
	ShopID       string    `json:"shop_id"`
	CustomerID   string    `json:"customer_id"`
	VIN          string    `json:"vin"`
	Make         string    `json:"make"`
	Model        string    `json:"model"`
	Year         int       `json:"year"`
	Color        string    `json:"color"`
	LicensePlate string    `json:"license_plate"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type RepairOrderStatus string

const (
	ROStatusCreated      RepairOrderStatus = "created"
	ROStatusDiagnosed    RepairOrderStatus = "diagnosed"
	ROStatusEstimateSent RepairOrderStatus = "estimate_sent"
	ROStatusApproved     RepairOrderStatus = "approved"
	ROStatusInProgress   RepairOrderStatus = "in_progress"
	ROStatusCompleted    RepairOrderStatus = "completed"
	ROStatusInvoiced     RepairOrderStatus = "invoiced"
	ROStatusClosed       RepairOrderStatus = "closed"
)

type RepairOrder struct {
	ID          string            `json:"id"`
	ShopID      string            `json:"shop_id"`
	CustomerID  string            `json:"customer_id"`
	VehicleID   string            `json:"vehicle_id"`
	Status      RepairOrderStatus `json:"status"`
	Description string            `json:"description"`
	Mileage     int               `json:"mileage"`
	CreatedBy   string            `json:"created_by"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type Estimate struct {
	ID            string    `json:"id"`
	ShopID        string    `json:"shop_id"`
	RepairOrderID string    `json:"repair_order_id"`
	Total         float64   `json:"total"`
	Status        string    `json:"status"`
	SentAt        *time.Time `json:"sent_at"`
	ApprovedAt    *time.Time `json:"approved_at"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type EstimateItem struct {
	ID         string  `json:"id"`
	EstimateID string  `json:"estimate_id"`
	Type       string  `json:"type"`
	Description string `json:"description"`
	Quantity   float64 `json:"quantity"`
	UnitPrice  float64 `json:"unit_price"`
	Total      float64 `json:"total"`
}

type InventoryPart struct {
	ID          string    `json:"id"`
	ShopID      string    `json:"shop_id"`
	Name        string    `json:"name"`
	SKU         string    `json:"sku"`
	Description string    `json:"description"`
	StockLevel  int       `json:"stock_level"`
	MinStock    int       `json:"min_stock"`
	UnitPrice   float64   `json:"unit_price"`
	VendorID    *string   `json:"vendor_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type LaborLog struct {
	ID            string    `json:"id"`
	ShopID        string    `json:"shop_id"`
	MechanicID    string    `json:"mechanic_id"`
	RepairOrderID string    `json:"repair_order_id"`
	Minutes       int       `json:"minutes"`
	Description   string    `json:"description"`
	ClockIn       time.Time `json:"clock_in"`
	ClockOut      *time.Time `json:"clock_out"`
	CreatedAt     time.Time `json:"created_at"`
}

type Invoice struct {
	ID            string    `json:"id"`
	ShopID        string    `json:"shop_id"`
	RepairOrderID string    `json:"repair_order_id"`
	Total         float64   `json:"total"`
	Paid          bool      `json:"paid"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Shop struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
