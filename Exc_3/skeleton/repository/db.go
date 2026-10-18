package repository

import (
	"ordersystem/model"
	"time"
)

type DatabaseHandler struct {
	// drinks represent all available drinks
	drinks []model.Drink
	// orders serves as order history
	orders []model.Order
}

// todo
func NewDatabaseHandler() *DatabaseHandler {
	// Init the drinks slice with some test data
	var drinks []model.Drink

	drinks = append(drinks, model.Drink{
		ID: 1,
		Name: "Beer",
		Price: 2.00,
		Description: "Hagenberger Gold",
})

	drinks = append(drinks, model.Drink{
		ID: 2,
		Name: "Spritzer",
		Price: 1.40,
		Description: "Wine and soda",
})

	drinks = append(drinks, model.Drink{
		ID: 3,
		Name: "Coffee",
		Price: 0.00,
		Description: "Mifare isn't that secure",
})

	// Init orders slice with some test data
	var orders []model.Order

	orders = append(orders, model.Order{
		DrinkID: 1,
		Amount: 3,
		CreatedAt: time.Now(),
})

	orders = append(orders, model.Order{
		DrinkID: 2,
		Amount: 2,
		CreatedAt: time.Now(),
})


	return &DatabaseHandler{
		drinks: drinks,
		orders: orders,
	}
}

func (db *DatabaseHandler) GetDrinks() []model.Drink {
	return db.drinks
}

func (db *DatabaseHandler) GetOrders() []model.Order {
	return db.orders
}

// todo
func (db *DatabaseHandler) GetTotalledOrders() map[uint64]uint64 {
	// calculate total orders
	// key = DrinkID, value = Amount of orders
	// totalledOrders map[uint64]uint64

	totalledOrders := make(map[uint64]uint64)

	for i := 0; i < len(db.orders); i++{
		orderNow := db.orders[i]
		drinkID := orderNow.DrinkID
		drinkNum := uint64(orderNow.Amount)

		if _, exists := totalledOrders[drinkID]; exists{
			totalledOrders[drinkID] = totalledOrders[drinkID] + drinkNum
		} else {
			totalledOrders[drinkID] = drinkNum

		}
}

	return totalledOrders
}

func (db *DatabaseHandler) AddOrder(order *model.Order) {
	// todo
	// add order to db.orders slice

	db.orders = append(db.orders, *order)
}
