// This example provides three small APIs users can call to try idempotency behavior.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	idemguard "github.com/mrmissong/idemguard"
)

// main configures separate idempotency middleware instances for payments,
// orders, and shipments.
func main() {
	paymentIdem := idemguard.MustNew(idemguard.Config{
		Store:       idemguard.NewMemoryStoreWithLimit(10000),
		TTL:         24 * time.Hour,
		ScopeFields: []string{"body.cart_id"},
		FingerprintFields: []string{
			"body.cart_id",
			"body.amount",
			"body.currency",
		},
	})

	orderIdem := idemguard.MustNew(idemguard.Config{
		Store:       idemguard.NewMemoryStoreWithLimit(10000),
		TTL:         24 * time.Hour,
		ScopeFields: []string{"body.checkout_session_id"},
		FingerprintFields: []string{
			"body.checkout_session_id",
			"body.items",
			"body.shipping_address_id",
		},
	})

	shipmentIdem := idemguard.MustNew(idemguard.Config{
		Store:       idemguard.NewMemoryStoreWithLimit(10000),
		TTL:         24 * time.Hour,
		ScopeFields: []string{"body.order_id"},
		FingerprintFields: []string{
			"body.order_id",
			"body.courier",
			"body.address_id",
		},
	})

	mux := http.NewServeMux()
	mux.Handle("POST /payments", paymentIdem.Middleware(http.HandlerFunc(createPayment)))
	mux.Handle("POST /orders", orderIdem.Middleware(http.HandlerFunc(createOrder)))
	mux.Handle("POST /shipments", shipmentIdem.Middleware(http.HandlerFunc(bookShipment)))

	log.Println("try the demo at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

// createPayment simulates a payment creation endpoint.
func createPayment(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusCreated, map[string]any{
		"payment_id": "pay_" + time.Now().Format("150405.000000"),
		"status":     "created",
	})
}

// createOrder simulates an order creation endpoint.
func createOrder(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusCreated, map[string]any{
		"order_id": "ord_" + time.Now().Format("150405.000000"),
		"status":   "created",
	})
}

// bookShipment simulates a shipment booking endpoint.
func bookShipment(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusCreated, map[string]any{
		"shipment_id": "shp_" + time.Now().Format("150405.000000"),
		"status":      "booked",
	})
}

// writeJSON keeps the demo handlers focused on their fake business response.
func writeJSON(w http.ResponseWriter, statusCode int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}
