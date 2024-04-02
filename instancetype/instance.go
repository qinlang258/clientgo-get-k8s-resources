package instancetype

const (
	EcsR6Xlarge = "ecs.r6.xlarge"
)

func GetInstanceTypeCost(instance string) float64 {
	var instanceCost float64

	switch instance {
	case "ecs.r6.xlarge":
		instanceCost = 500.00
		return instanceCost
	}

	return instanceCost
}
