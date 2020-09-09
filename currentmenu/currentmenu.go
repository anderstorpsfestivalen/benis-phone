package currentmenu

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/jmoiron/sqlx/types"
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

type Recipe struct {
	ID                 int64        `db:"id"  json:"-"`
	RecipeID           string       `db:"recipeid"`
	Name               string       `db:"name"`
	Image              string       `db:"image"`
	PriceOverride      float64      `db:"priceoverride"  json:"-"`
	Enabled            sql.NullBool `db:"enabled"`
	Variations         []string
	UnpackedVariations map[string]Variation
}

type Variation struct {
	ID          int64          `db:"id"  json:"-"`
	RecipeID    string         `db:"recipeid"`
	VariationID string         `db:"variationid"`
	Price       float64        `db:"price"`
	Name        string         `db:"name"`
	Recipe      types.JSONText `db:"recipe"`
}

type MenuAPIResopnse struct {
	Ingredients map[string]Ingredient
	Recipes     map[string]Recipe
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
			fmt.Println(ingredient.Name, ingredient.Price)
		}
	}
	for _, recipe := range s.Recipes {
		if recipe.Enabled.Bool {
			fmt.Println(recipe.Name)

			if len(recipe.Variations) > 0 {

				if variation, ok := recipe.UnpackedVariations[recipe.Variations[0]]; ok {
					fmt.Println(variation.Price)
				}
			}
		}
	}

	message := "hej"
	return message

}
