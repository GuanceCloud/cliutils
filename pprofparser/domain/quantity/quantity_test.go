// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

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
