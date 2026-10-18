package rest

import (
	"net/http"
	"ordersystem/repository"
	"encoding/json"
	"ordersystem/model"
	"github.com/go-chi/render"
)

// GetMenu 			godoc
// @tags 			Menu
// @Description 	Returns the menu of all drinks
// @Produce  		json
// @Success 		200 {array} model.Drink
// @Router 			/api/menu [get]
func GetMenu(db *repository.DatabaseHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// todo
		// get slice from db
		drinks := db.GetDrinks()

		render.Status(r, http.StatusOK)
		render.JSON(w, r, drinks)
	}
}

// todo create GetOrders /api/order/all
func GetOrders(db *repository.DatabaseHandler) http.HandlerFunc{
	return func(w http.ResponseWriter, r *http.Request){
		orders := db.GetOrders()

		render.Status(r, http.StatusOK)
		render.JSON(w, r, orders)
	}

}

// todo create GetOrdersTotal /api/order/total
func GetOrdersTotal(db *repository.DatabaseHandler) http.HandlerFunc{
	return func(w http.ResponseWriter, r *http.Request){
		totalledOrders := db.GetTotalledOrders()

		render.Status(r, http.StatusOK)
		render.JSON(w, r, totalledOrders)
	}
}


// PostOrder 		godoc
// @tags 			Order
// @Description 	Adds an order to the db
// @Accept 			json
// @Param 			b body model.Order true "Order"
// @Produce  		json
// @Success 		200
// @Failure     	400
// @Router 			/api/order [post]
func PostOrder(db *repository.DatabaseHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// todo
		// declare empty order struct
		var emptyOrder model.Order

		// err := json.NewDecoder(r.Body).Decode(&<your-order-struct>)
		err := json.NewDecoder(r.Body).Decode(&emptyOrder)
		if err != nil{
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, map[string]string{"error": "Bad request 400"})
			return
		}
		
		db.AddOrder(&emptyOrder)
		render.Status(r, http.StatusOK)
		render.JSON(w, r, "ok")
	}
}
