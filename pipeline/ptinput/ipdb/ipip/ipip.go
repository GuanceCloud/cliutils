package ipip

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/GuanceCloud/cliutils/logger"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/ipdb"
	ipipdb "github.com/ipipdotnet/ipdb-go"
)

var (
	_   ipdb.IPdb = (*IPIP)(nil)
	log           = logger.DefaultSLogger("ipip")
)

func InitLog() {
	log = logger.SLogger("iploc")
}

type IPIP struct {
	db   *ipipdb.City
	lang string
}

func (ipip *IPIP) Init(dataDir string, config map[string]string) {
	var ipipFile string
	if f, ok := config["ipip_file"]; ok {
		ipipFile = filepath.Join(dataDir, f)
	} else {
		dLi, err := os.ReadDir(dataDir)
		if err != nil {
			return
		}
		for _, e := range dLi {
			name := e.Name()
			if filepath.Ext(name) == ".ipdb" {
				ipipFile = filepath.Join(dataDir, name)
				log.Warnf(
					"no file was specified, the file in the `%s` will be used: `%s`",
					dataDir, ipipFile)
				break
			}
		}
	}

	if ipipFile == "" {
		log.Error("no file was specified")
		return
	} else {
		log.Infof("load ip database from file `%s`", ipipFile)
	}

	if db, err := ipipdb.NewCity(ipipFile); err != nil {
		log.Error(err)
		return
	} else {
		ipip.db = db
	}

	langLi := ipip.db.Languages()
	if lang, ok := config["ipip_language"]; ok {
		var br bool
		for i := range langLi {
			if lang == langLi[i] {
				ipip.lang = lang
				br = true
				break
			}
		}
		if !br {
			log.Errorf("supported languages include `%v`, actual specified is `%s`",
				strings.Join(langLi, ", "), lang)
		}
	}

	if ipip.lang == "" && len(langLi) > 0 {
		ipip.lang = langLi[0]
		log.Warnf("use `%s` from the provided language list `%s`",
			ipip.lang, strings.Join(langLi, ", "),
		)
	}
}

func (ipip *IPIP) SearchIsp(ip string) string {
	if ipip.db == nil {
		return "unknown"
	}
	c, err := ipip.db.FindInfo(ip, ipip.lang)
	if err != nil {
		return "unknown"
	}
	return c.IspDomain
}

func (ipip *IPIP) Geo(ip string) (*ipdb.IPdbRecord, error) {
	if ipip.db == nil {
		return nil, fmt.Errorf("IP database not found")
	}

	c, err := ipip.db.FindInfo(ip, ipip.lang)
	if err != nil {
		return nil, err
	}

	rec := &ipdb.IPdbRecord{
		Region:   c.RegionName,
		City:     c.CityName,
		Isp:      c.IspDomain,
		Timezone: c.Timezone,
		Areacode: c.AreaCode,
	}

	if rec.Isp == "" {
		rec.Isp = "unknown"
	}

	if c.CountryCode != "" {
		rec.Country = c.CountryCode
	} else {
		rec.Country = c.CountryName
	}

	return rec, nil
}
