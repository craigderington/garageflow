# GarageFlow — Product Plan: The Killer App for Shops & Grease Monkeys

> How we go from "a working shop-management app" to the one a mechanic refuses to
> work without. Grounded in what's already built (see [[project-overview]]) and in
> what actually wins in the auto-repair software market.

---

## 1. The thesis

The auto-repair software market (Tekmetric, Shop-Ware, ShopMonkey, AutoLeap, Mitchell1)
is won on one move: **turn the inspection into the sales pitch.** The shop that can
show a customer a photo of their cracked brake pad and get a thumbs-up from the
customer's phone in 90 seconds makes more money and earns more trust. Everything else
(ROs, invoicing, scheduling) is table stakes.

But there's a split we must respect:

- **The buyer is the shop owner** — cares about revenue, average repair order (ARO),
  throughput, reviews, and not getting ripped off by software that takes a week to learn.
- **The daily user is the tech / "grease monkey"** — greasy hands, loud bay, skeptical
  of corporate software, judges an app in 10 seconds. If it's slow or makes him type a
  lot, he abandons it and goes back to the paper clipboard. **The tech's adoption is the
  whole game** — if the techs use it, the data is real, and the owner gets the reporting.

We win by being **the fastest thing in the bay** and **the most trustworthy thing on the
customer's phone.**

---

## 2. Where we are (the foundation is real)

Already built and tested (≈55 E2E tests green on the docker stack):

- Repair orders w/ status lifecycle, customers, vehicles (CRUD + detail pages)
- Estimates → send → approve → **pay (Stripe)**
- Inventory + restock + low-stock alerts
- Labor clock-in/out, scheduling + bays
- Customer portal endpoints, realtime event hub (WebSocket)
- **Photos on repair orders (MinIO object storage)** ← this is the seed of the killer feature
- Email (Mailgun), dark "Garage Industrial" UI, marketing site, multi-tenant

**Key insight:** we already have RO photos + a customer portal + estimates + payments.
The killer feature (DVI) is ~80% infrastructure we've already laid. We are much closer
than the feature list suggests.

---

## 3. The wedge: Digital Vehicle Inspection (DVI)

This is the #1 feature and our entry into "killer app" status. Build this next.

**The flow:**
1. Tech opens an RO on a tablet, taps **Start Inspection**.
2. Walks the car against a **template** (e.g. "Courtesy Check": brakes, tires, fluids,
   belts, battery, lights…). Each item gets a tap: 🟢 Good / 🟡 Attention / 🔴 Now.
3. For yellow/red items, snap a **photo or 15-sec video** (we already store media in MinIO)
   and add a one-tap or voice note.
4. Tap **Send to customer** → customer gets an **SMS link** to a mobile inspection report:
   color-coded findings, the actual photos of *their* car, and an itemized estimate.
5. Customer taps **Approve** on the work they want → it flows straight onto the RO and
   estimate → tech sees "approved, go" in real time.

**Why it's the killer feature:**
- **Techs love it** — structured, fast, photo-first, minimal typing. It respects the craft.
- **Customers trust it** — "here's a picture of your actual brake pad" kills the
  "mechanics are scammers" objection. This is the single biggest ARO driver in the industry.
- **Owners get the money** — transparent upsell + a record of declined work to follow up on.

**What we need to build:** inspection templates (configurable checklists), an inspection
record tied to an RO, the tablet inspection UI, the public mobile report page (extend the
customer portal), line-item → estimate linkage, and SMS delivery.

**Effort given our stack:** Medium. Media + portal + estimates + realtime already exist.
New: `inspections` + `inspection_items` tables, a templates table, the tablet UI, the
public report view, and Twilio for SMS.

---

## 4. The "grease monkey" delight layer

These are the things that make a tech *choose* the app over the clipboard. Cheap to build,
disproportionate to adoption. We already have the dark theme and photo storage — lean in.

- **Tablet-first bay UI** — big touch targets, glanceable, works with dirty/gloved hands.
  A dedicated "Bay Mode" route that's not the desktop admin UI.
- **VIN scan + license-plate lookup** — camera scans the VIN barcode (or plate) → auto-fills
  year/make/model/engine. No typing 17 characters with greasy thumbs. (VIN decode via free
  NHTSA vPIC API; plate lookup via a paid provider later.)
- **Voice-to-text notes** — hold a button, talk, done. Mechanics narrate; they don't type.
- **Photo/video first everywhere** — already have the storage; make the camera one tap from
  any RO.
- **"My Work Today" board** — the tech logs in and sees *only* his assigned jobs, in order,
  with a giant "Clock In" button. Not a CRM. Not a dashboard. Just "what do I turn next."
- **Offline tolerance** — bays have bad wifi. Queue photo uploads and status changes,
  sync when reconnected. (PWA + local queue.)
- **It's fast.** Sub-second everything. A tech will forgive missing features; he will not
  forgive lag. This is a first-class requirement, not a nice-to-have.

---

## 5. Revenue & retention engine (what keeps the shop paying)

Modern shops switch software for these. They're the moat after DVI.

- **Two-way SMS** (Twilio) — texting is how shops talk to customers now. Status updates
  ("your car's ready"), estimate approvals, review requests. This is arguably co-#1 with DVI.
- **Declined / deferred work tracking** — every red/yellow item the customer skipped becomes
  a follow-up: "You declined rear brakes 3 months ago — still recommended." Automated
  reminders. This is *free money* the shop is currently leaving on the table.
- **Parts ordering integration** — order parts from PartsTech / Nexpart / WorldPac /
  RockAuto directly from the estimate; price + availability inline. Removes the biggest
  daily time-sink. (Big, partner-gated, but a category-definer.)
- **Labor time guides** — integrated labor times (Motor/Mitchell) so estimates aren't guessed.
  Licensing-heavy; start with a shop's own saved "canned jobs / packages" (free, high value).
- **Review generation** — after pickup, auto-text a Google review link. Reputation is how
  shops get customers; closing the loop here is sticky.
- **Service reminders & maintenance schedules** — "due for an oil change" texts by
  mileage/time. Brings cars back. Recurring revenue for the shop = retention for us.
- **Financing at checkout** — buy-now-pay-later for big repairs (we already have Stripe;
  add Stripe/affirm-style financing). Bigger tickets get approved.

---

## 6. Owner's command center (the buyer's reasons to buy)

- **Dispatch board** — kanban of every RO by status across bays/techs (already scoped as
  "premium"). The owner's at-a-glance "where is everything."
- **The numbers that matter** — ARO, car count, gross profit per RO, tech productivity
  (billed vs. actual hours), effective labor rate, parts margin. Shops live and die by ARO.
- **Tech performance & efficiency** — hours billed vs. clocked (we already track labor).
- **Capacity / scheduling intelligence** — don't overbook the bays; show realistic promise
  times.
- **Multi-location** — once a shop grows or a small chain adopts, this is the upsell ceiling.
- **QuickBooks / accounting export** — non-negotiable for the owner's books. Table stakes
  for switching.

---

## 7. Integrations & the data moat

Priority order (each one is a switching-cost brick):

1. **Twilio** (SMS) — needed for DVI + status + reviews. Do first.
2. **NHTSA vPIC** (free VIN decode) — instant vehicle data, zero cost.
3. **CARFAX / service history** — "QuickVIN" + putting service records into CARFAX is a
   trust + marketing signal shops want.
4. **PartsTech / parts catalogs** — the daily-workflow lock-in.
5. **QuickBooks** — the accounting lock-in.
6. **Motor/Mitchell labor & repair data** — the estimating lock-in (expensive; later).

The moat compounds: inspection history + photos + declined work + service reminders per
vehicle means **the longer a shop uses us, the more valuable their own data makes us.**

---

## 8. Roadmap — Now / Next / Later

### NOW (the wedge — next 1–2 build cycles)
- **DVI v1**: inspection templates, tablet inspection flow, photo/video capture, public
  mobile report, approve-to-estimate. *(builds on photos + portal + estimates)*
- **Twilio SMS v1**: send the inspection/estimate link, "ready for pickup," approvals.
  *(mirror the Mailgun/Stripe dev-mode pattern: no-key → log/stub so E2E stays green)*
- **VIN decode (NHTSA)** on the vehicle form. *(tiny, high delight)*
- **"My Work Today" tech board** + a tablet-friendly Bay Mode shell.

### NEXT
- **Declined/deferred work** tracking + automated follow-ups.
- **Two-way SMS inbox** + review-request automation.
- **Dispatch board** (kanban) for owners.
- **Canned jobs / service packages** (shop-defined labor+parts bundles).
- **Owner KPIs**: ARO, car count, tech efficiency, effective labor rate.

### LATER
- Parts ordering integration (PartsTech/WorldPac).
- QuickBooks export.
- Licensed labor guides (Motor/Mitchell).
- Service reminders / maintenance schedules + financing at checkout.
- Multi-location, capacity forecasting, offline PWA hardening.

---

## 9. Recommended immediate next 3 builds

1. **DVI v1** — the wedge. Everything else is table stakes; this is why they switch.
2. **Twilio SMS** — DVI is half as powerful without the text-to-customer delivery.
3. **VIN scan/decode + Bay Mode tablet shell** — the "grease monkeys love it" proof point,
   cheap to ship, sets up every future tech-facing feature.

Each ships with the same rigor we've used: graceful no-key dev mode for external services,
full Playwright E2E, green on the docker stack.

---

## 10. One-line positioning

> **GarageFlow — run the bay, not the paperwork.** The shop app techs actually use:
> photo inspections your customers trust, approvals from their phone, and every repair
> on one screen.
