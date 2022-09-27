package dialtesting

import (
	"fmt"
	"testing"
)

func TestTraceroute(t *testing.T) {
	routes, err := TracerouteIP("180.101.49.13", &TracerouteOption{
		Hops:  30,
		Retry: 3,
	})

	fmt.Println(routes, err)

	for index, route := range routes {
		fmt.Printf("%d ", index)
		for _, item := range route.Items {
			fmt.Printf("%s %f ", item.IP, item.ResponseTime)
		}
		fmt.Printf(" total: %d, failed: %d, loss: %f, avg: %f, max: %f, min: %f, std: %f\n", route.Total, route.Failed, route.Loss, route.AvgCost, route.MaxCost, route.MinCost, route.StdCost)
	}
}
