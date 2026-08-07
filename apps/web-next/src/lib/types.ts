export type Role = "owner" | "admin" | "service_writer" | "technician";

export type RepairOrderStatus =
  | "created"
  | "diagnosed"
  | "estimate_sent"
  | "approved"
  | "in_progress"
  | "completed"
  | "invoiced"
  | "closed";

export interface User {
  id: string;
  shop_id: string;
  email: string;
  role: Role;
  name: string;
  specialities?: string;
  hourly_rate?: number;
}

export interface Customer {
  id: string;
  shop_id: string;
  name: string;
  phone: string;
  email: string;
  notes: string;
}

export interface Vehicle {
  id: string;
  shop_id: string;
  customer_id: string;
  vin: string;
  make: string;
  model: string;
  year: number;
  color: string;
  license_plate: string;
}

export interface RepairOrder {
  id: string;
  shop_id: string;
  customer_id: string;
  vehicle_id: string;
  status: RepairOrderStatus;
  description: string;
  mileage: number;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface Estimate {
  id: string;
  shop_id: string;
  repair_order_id: string;
  total: number;
  status: string;
  sent_at: string | null;
  approved_at: string | null;
}

export interface EstimateItem {
  id: string;
  estimate_id: string;
  type: "part" | "labor" | "fee";
  description: string;
  quantity: number;
  unit_price: number;
  total: number;
}

export interface InventoryPart {
  id: string;
  shop_id: string;
  name: string;
  sku: string;
  description: string;
  stock_level: number;
  min_stock: number;
  unit_price: number;
  vendor_id: string | null;
}

export interface LaborLog {
  id: string;
  shop_id: string;
  mechanic_id: string;
  repair_order_id: string;
  minutes: number;
  description: string;
  clock_in: string;
  clock_out: string | null;
}

export interface Bay {
  id: string;
  shop_id: string;
  name: string;
  active: boolean;
}

export interface Schedule {
  id: string;
  shop_id: string;
  bay_id: string;
  repair_order_id: string;
  technician_id: string;
  start_time: string;
  end_time: string;
}
