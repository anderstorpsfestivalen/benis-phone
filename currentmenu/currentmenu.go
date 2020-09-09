package currentmenu

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Ingredient struct {
	ID            int64              `db:"id" json:"-"`
	IngredientID  string             `db:"ingredientid"`
	Name          string             `db:"name"`
	Image         string             `db:"image"`
	Enabled       sql.NullBool       `db:"enabled"`
	Price         float64            `db:"price"`
	ABV           float64            `db:"abv"`
	ServingSize   int64              `db:"servingsize"`
	ContainerSize int64              `db:"containersize"`
	Points        int64              `db:"points"`
	IsFluid       sql.NullBool       `db:"isfluid" json:"-"`
	Category      IngredientCategory `db:"category"`
}

type IngredientCategory string

const (
	Beer       IngredientCategory = "beer"
	FoxBeer    IngredientCategory = "foxbeer"
	Sprit      IngredientCategory = "sprit"
	Cider      IngredientCategory = "cider"
	Wine       IngredientCategory = "wine"
	Mixer      IngredientCategory = "mixer"
	Consumable IngredientCategory = "consumable"
	Other      IngredientCategory = "other"
	Hidden     IngredientCategory = "hidden"
)

type MenuAPIResopnse struct {
	Ingredients map[string]Ingredient
}

func ListItems() string {

	s := MenuAPIResopnse{}
	res, err := http.Get("https://anderstorpsfestivalen.se/api/menu")
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	json.Unmarshal(body, &s)

	for _, ingredient := range s.Ingredients {
		if ingredient.Enabled.Bool {
			fmt.Println(ingredient.Name)
		}
	}

	message := "hej"
	return message

}
