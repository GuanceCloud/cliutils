package quantity

import (
	"fmt"
	"testing"
)

func TestQuantity_String(t *testing.T) {
	quantity := KiloByte.Quantity(2000)
	fmt.Println(quantity)

	fmt.Println(quantity.DoubleValueIn(MegaByte))
	fmt.Println(quantity.IntValueIn(MegaByte))
}
