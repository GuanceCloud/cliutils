package pipeline

import (
	"github.com/GuanceCloud/cliutils/pipeline/manager"
	"github.com/GuanceCloud/cliutils/pipeline/manager/relation"
	"github.com/GuanceCloud/cliutils/pipeline/offload"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/funcs"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/ipdb/geoip"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/ipdb/iploc"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/plmap"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/refertable"
	"github.com/GuanceCloud/cliutils/pipeline/stats"
)

func InitLog() {
	// pipeline scripts manager
	manager.InitLog()
	// scripts relation
	relation.InitLog()

	// pipeline offload
	offload.InitLog()

	// all ptinputs's package

	// inner pl functions
	funcs.InitLog()
	// ip db
	iploc.InitLog()
	geoip.InitLog()
	// pipeline map
	plmap.InitLog()
	// refertable
	refertable.InitLog()

	// stats
	stats.InitLog()
}
