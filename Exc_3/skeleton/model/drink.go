package model

type Drink struct {
	ID uint64 `json:"id"`
	Description string `json:"description"`
	Name string `json:"name"`
	Price float64 `json:"price"`
}
