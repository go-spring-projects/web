package models

import "time"

type Order struct {
	ID       int       `json:"id"`
	PetID    int       `json:"petId"`
	Quantity int       `json:"quantity"`
	ShipDate time.Time `json:"shipDate"`
	Status   string    `json:"status"`
	Complete bool      `json:"complete"`
}
