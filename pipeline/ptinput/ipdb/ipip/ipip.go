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

type cfg struct {
	dir  string
	file string
	lang string
}
type IPIP struct {
	db   *ipipdb.City
	lang string
}

const (
	CfgIPIPFile     = "ipip_file"
	CfgIPIPLanguage = "ipip_language"
)

func (ipip *IPIP) Init(dataDir string, config map[string]string) {
	cfg := cfg{
		dir: dataDir,
	}

	if v, ok := config[CfgIPIPFile]; ok {
		cfg.file = v
	}

	if v, ok := config[CfgIPIPLanguage]; ok {
		cfg.lang = v
	}
	ipip.db, ipip.lang = newIPDB(cfg)
}

func newIPDB(cfg cfg) (*ipipdb.City, string) {
	var fp string
	if cfg.file != "" {
		fp = filepath.Join(cfg.dir, cfg.file)
	} else {
		dLi, err := os.ReadDir(cfg.dir)
		if err != nil {
			log.Error(err)
			return nil, ""
		}
		for _, e := range dLi {
			name := e.Name()
			if filepath.Ext(name) == ".ipdb" {
				fp = filepath.Join(cfg.dir, name)
				log.Warnf(
					"no file was specified, the file in the `%s` will be used: `%s`",
					cfg.dir, fp)
				break
			}
		}
	}

	if fp == "" {
		log.Error("no file was specified")
		return nil, ""
	} else {
		log.Infof("load ip database from file `%s`", fp)
	}

	var ipdb *ipipdb.City
	if db, err := ipipdb.NewCity(fp); err != nil {
		log.Error(err)
		return nil, ""
	} else {
		ipdb = db
	}

	return ipdb, checkLang(cfg.lang, ipdb)
}

func checkLang(lang string, ipdb *ipipdb.City) string {
	// var lang string
	langLi := ipdb.Languages()
	if lang != "" {
		for i := range langLi {
			if lang == langLi[i] {
				return lang
			}
		}
		log.Errorf("supported languages include `%v`, actual specified is `%s`",
			strings.Join(langLi, ", "), lang)
	}

	if len(langLi) > 0 {
		lang = langLi[0]
		log.Warnf("use `%s` from the provided language list `%s`",
			lang, strings.Join(langLi, ", "),
		)
		return lang
	}

	return ""
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
