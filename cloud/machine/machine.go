package machine

import (
	"fmt"

	"github.com/kelda/kelda/blueprint"
	"github.com/kelda/kelda/db"
)

// Description describes a VM type offered by a cloud provider.
type Description struct {
	Size   string
	Price  float64
	RAM    float64
	CPU    int
	Disk   string
	Region string
}

// ChooseSize returns an acceptable machine size for the given provider that fits the
// provided ram, cpu, and price constraints.
func ChooseSize(provider db.ProviderName, ram, cpu blueprint.Range) string {
	switch provider {
	case db.Amazon:
		return chooseBestSize(amazonDescriptions, ram, cpu)
	case db.DigitalOcean:
		return chooseBestSize(digitalOceanDescriptions, ram, cpu)
	case db.Google:
		return chooseBestSize(googleDescriptions, ram, cpu)
	case db.Vagrant:
		return vagrantSize(ram, cpu)
	default:
		panic(fmt.Sprintf("Unknown Cloud Provider: %s", provider))
	}
}

func chooseBestSize(descriptions []Description, ram, cpu blueprint.Range) string {
	var best Description
	for _, d := range descriptions {
		if ram.Accepts(d.RAM) &&
			cpu.Accepts(float64(d.CPU)) &&
			(best.Size == "" || d.Price < best.Price) {
			best = d
		}
	}
	return best.Size
}

func vagrantSize(ramRange, cpuRange blueprint.Range) string {
	ram := ramRange.Min
	if ram < 1 {
		ram = 1
	}

	cpu := cpuRange.Min
	if cpu < 1 {
		cpu = 1
	}
	return fmt.Sprintf("%g,%g", ram, cpu)
}
